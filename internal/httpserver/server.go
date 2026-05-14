package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"heimdall/internal/app"
	"heimdall/internal/domain"
)

func New(log *slog.Logger, svc *app.Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		if !svc.Ready() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("POST /fraud-score", func(w http.ResponseWriter, r *http.Request) {
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
	})
	return mux
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
