package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"heimdall/internal/app"
	"heimdall/internal/domain"
	"heimdall/internal/metrics"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func New(log *slog.Logger, svc *app.Service, reg prometheus.Gatherer, fs *metrics.FraudScore) http.Handler {
	mux := http.NewServeMux()
	if reg != nil {
		mux.Handle("GET /metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	}
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		if !svc.Ready() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("POST /fraud-score", fraudScoreHandler(log, svc, fs))
	return mux
}

func fraudScoreHandler(log *slog.Logger, svc *app.Service, fs *metrics.FraudScore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var deferObs func()
		if fs != nil {
			sw := &statusRecorder{ResponseWriter: w}
			w = sw
			start := time.Now()
			deferObs = func() {
				fs.HandlerSeconds.Observe(time.Since(start).Seconds())
				fs.Responses.WithLabelValues(httpStatusClass(sw.status())).Inc()
			}
		}
		if deferObs != nil {
			defer deferObs()
		}

		if !svc.Ready() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		defer func() { _ = r.Body.Close() }()
		dec := json.NewDecoder(r.Body)
		var req domain.FraudScoreRequest
		if err := dec.Decode(&req); err != nil {
			log.Warn("decode fraud-score", "err", err)
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		ctx := r.Context()
		out, err := svc.Score(ctx, req)
		if err != nil {
			log.Error("score", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		if err := enc.Encode(out); err != nil {
			log.Warn("encode response", "err", err)
		}
	}
}

type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.code = code
	sr.ResponseWriter.WriteHeader(code)
}

func (sr *statusRecorder) Write(b []byte) (int, error) {
	if sr.code == 0 {
		sr.code = http.StatusOK
	}
	return sr.ResponseWriter.Write(b)
}

func (sr *statusRecorder) status() int {
	if sr.code == 0 {
		return http.StatusOK
	}
	return sr.code
}

func httpStatusClass(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500:
		return "5xx"
	default:
		return "other"
	}
}

// DefaultServer retorna servidor com timeouts defensivos (camada HTTP).
func DefaultServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}
