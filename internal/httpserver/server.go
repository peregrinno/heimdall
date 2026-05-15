package httpserver

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"heimdall/internal/app"
	"heimdall/internal/domain"
	"heimdall/internal/metrics"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	bodyApprovedTrue  = []byte(`{"approved":true,"fraud_score":`)
	bodyApprovedFalse = []byte(`{"approved":false,"fraud_score":`)
	bodyClose         = []byte("}")

	contentTypeJSON = []string{"application/json"}

	respPool = sync.Pool{New: func() any { b := make([]byte, 0, 64); return &b }}
	reqPool  = sync.Pool{New: func() any { return new(domain.FraudScoreRequest) }}
	bodyPool = sync.Pool{New: func() any { b := make([]byte, 0, 2048); return &b }}
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
	hasMetrics := fs != nil
	return func(w http.ResponseWriter, r *http.Request) {
		var start time.Time
		var sw *statusRecorder
		if hasMetrics {
			sw = &statusRecorder{ResponseWriter: w}
			w = sw
			start = time.Now()
			defer func() {
				fs.HandlerSeconds.Observe(time.Since(start).Seconds())
				fs.Responses.WithLabelValues(httpStatusClass(sw.status())).Inc()
			}()
		}

		if !svc.Ready() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}

		req := reqPool.Get().(*domain.FraudScoreRequest)
		req.LastTransaction = nil
		req.Customer.KnownMerchants = req.Customer.KnownMerchants[:0]
		bodyPtr := bodyPool.Get().(*[]byte)
		defer func() {
			_ = r.Body.Close()
			reqPool.Put(req)
			*bodyPtr = (*bodyPtr)[:0]
			bodyPool.Put(bodyPtr)
		}()

		body := (*bodyPtr)[:cap(*bodyPtr)]
		nRead, err := readAllInto(r.Body, body)
		if err != nil {
			log.Warn("read body", "err", err)
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		body = body[:nRead]
		*bodyPtr = body

		if err := json.Unmarshal(body, req); err != nil {
			log.Warn("decode fraud-score", "err", err)
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		out, err := svc.Score(r.Context(), *req)
		if err != nil {
			log.Error("score", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		bufPtr := respPool.Get().(*[]byte)
		buf := (*bufPtr)[:0]
		if out.Approved {
			buf = append(buf, bodyApprovedTrue...)
		} else {
			buf = append(buf, bodyApprovedFalse...)
		}
		buf = strconv.AppendFloat(buf, out.FraudScore, 'f', -1, 64)
		buf = append(buf, bodyClose...)

		h := w.Header()
		h["Content-Type"] = contentTypeJSON
		_, _ = w.Write(buf)

		*bufPtr = buf
		respPool.Put(bufPtr)
	}
}

func readAllInto(r io.Reader, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := r.Read(buf[total:])
		total += n
		if err != nil {
			if err == io.EOF {
				return total, nil
			}
			return total, err
		}
	}
	overflow := [128]byte{}
	for {
		n, err := r.Read(overflow[:])
		total += n
		if err != nil {
			if err == io.EOF {
				break
			}
			return total, err
		}
		if n == 0 {
			break
		}
	}
	return total, nil
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

type ServerTimeouts struct {
	ReadHeader time.Duration
	Read       time.Duration
	Write      time.Duration
	Idle       time.Duration
}

func DefaultServerTimeouts() ServerTimeouts {
	return ServerTimeouts{
		ReadHeader: 5 * time.Second,
		Read:       10 * time.Second,
		Write:      10 * time.Second,
		Idle:       120 * time.Second,
	}
}

func NewServer(addr string, h http.Handler, t ServerTimeouts) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: t.ReadHeader,
		ReadTimeout:       t.Read,
		WriteTimeout:      t.Write,
		IdleTimeout:       t.Idle,
	}
}

func DefaultServer(addr string, h http.Handler) *http.Server {
	return NewServer(addr, h, DefaultServerTimeouts())
}
