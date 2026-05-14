package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"heimdall/internal/app"
	"heimdall/internal/config"
	"heimdall/internal/httpserver"
	"heimdall/internal/metrics"

	"github.com/prometheus/client_golang/prometheus"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	listen := getenv("LISTEN", ":8080")
	dataDir := getenv("DATA_DIR", "./data")
	refPath := getenv("REFERENCE_PATH", filepath.Join(dataDir, "references.rbin"))

	normPath := filepath.Join(dataDir, "normalization.json")
	mccPath := filepath.Join(dataDir, "mcc_risk.json")

	norm, err := config.LoadNormalization(normPath)
	if err != nil {
		log.Error("normalização", "path", normPath, "err", err)
		os.Exit(1)
	}
	mcc, err := config.LoadMCCRisk(mccPath)
	if err != nil {
		log.Error("mcc_risk", "path", mccPath, "err", err)
		os.Exit(1)
	}

	log.Info("carregando referências", "path", refPath)
	knnMode := getenv("KNN_MODE", "exact")
	idx, err := app.OpenReferenceIndex(refPath, app.ReferenceIndexConfig{
		KNNMode:    knnMode,
		IVFPath:    getenv("REFERENCE_IVF_PATH", ""),
		IVFProbes:  getenvInt("KNN_NPROBE", 24),
		IVFMaxCand: getenvInt("KNN_IVF_MAX_CANDIDATES", 300_000),
	})
	if err != nil {
		log.Error("referências", "path", refPath, "err", err)
		os.Exit(1)
	}
	log.Info("referências prontas", "n", idx.Len(), "knn_mode", knnMode)

	reg := prometheus.NewRegistry()
	fs := metrics.RegisterFraudScore(reg)

	svc := app.NewService(log, norm, mcc, idx, fs.KNNDuration)
	defer func() {
		if err := svc.Close(); err != nil {
			log.Warn("fechar serviço", "err", err)
		}
	}()
	h := httpserver.New(log, svc, reg, fs)
	srv := httpserver.NewServer(listen, h, httpserver.ServerTimeouts{
		ReadHeader: durationSecEnv("HTTP_READ_HEADER_TIMEOUT_SEC", 5),
		Read:       durationSecEnv("HTTP_READ_TIMEOUT_SEC", 120),
		Write:      durationSecEnv("HTTP_WRITE_TIMEOUT_SEC", 120),
		Idle:       durationSecEnv("HTTP_IDLE_TIMEOUT_SEC", 60),
	})

	go func() {
		log.Info("http escutando", "addr", listen)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("servidor", "err", err)
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getenvInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func durationSecEnv(k string, defSecs int) time.Duration {
	return time.Duration(getenvInt(k, defSecs)) * time.Second
}
