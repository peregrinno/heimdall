package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"heimdall/internal/app"
	"heimdall/internal/config"
	"heimdall/internal/httpserver"
	"heimdall/internal/metrics"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	embeddedReferencesRBin = "/app/data/references.rbin"
	defaultMinReferences   = 2_000_000
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	listen := getenv("LISTEN", ":8080")
	dataDir := getenv("DATA_DIR", "./data")
	refPath := pickReferencePath(getenv("REFERENCE_PATH", filepath.Join(dataDir, "references.rbin")), log)

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
	knnMode := getenv("KNN_MODE", "auto")
	idx, err := app.OpenReferenceIndex(refPath, app.ReferenceIndexConfig{
		KNNMode:    knnMode,
		IVFPath:    getenv("REFERENCE_IVF_PATH", ""),
		IVFProbes:  getenvInt("KNN_NPROBE", 16),
		IVFMaxCand: getenvInt("KNN_IVF_MAX_CANDIDATES", 10_000),
	})
	if err != nil {
		log.Error("referências", "path", refPath, "err", err)
		os.Exit(1)
	}
	effective := idx.KNNMode()
	if knnMode == "ivf" && effective != "ivf" {
		log.Warn("REFERENCE_IVF ausente ou inválido; usando KNN exato", "ivf", ivfPathFor(refPath, getenv("REFERENCE_IVF_PATH", "")))
	}
	log.Info("referências prontas", "n", idx.Len(), "knn_mode", knnMode, "knn_effective", effective)

	if getenv("WARMUP", "1") == "1" {
		t0 := time.Now()
		idx.Warmup()
		log.Info("warmup mmap concluído", "elapsed_ms", time.Since(t0).Milliseconds())
	}

	minRefs := getenvInt("MIN_REFERENCES", defaultMinReferences)
	if minRefs > 0 && idx.Len() < minRefs {
		log.Error(
			"dataset de referências insuficiente para a Rinha",
			"n", idx.Len(),
			"min", minRefs,
			"path", refPath,
			"hint", "gere data/references.rbin a partir de references.json.gz e monte em /data; use MIN_REFERENCES=0 ou ALLOW_SMALL_REFERENCES=1 só em desenvolvimento",
		)
		os.Exit(1)
	}

	if os.Getenv("HEIMDALL_DISABLE_GC") == "1" {
		debug.SetGCPercent(-1)
	}
	if s := os.Getenv("HEIMDALL_MEM_LIMIT_BYTES"); s != "" {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
			debug.SetMemoryLimit(n)
		}
	}

	var reg prometheus.Registerer = prometheus.NewRegistry()
	var fs *metrics.FraudScore
	if getenv("METRICS", "0") == "1" {
		g := prometheus.NewRegistry()
		reg = g
		fs = metrics.RegisterFraudScore(g)
	}

	var knnObs prometheus.Observer
	if fs != nil {
		knnObs = fs.KNNDuration
	}

	svc := app.NewService(log, norm, mcc, idx, knnObs)
	defer func() {
		if err := svc.Close(); err != nil {
			log.Warn("fechar serviço", "err", err)
		}
	}()
	var gatherer prometheus.Gatherer
	if g, ok := reg.(prometheus.Gatherer); ok {
		gatherer = g
	}
	h := httpserver.New(log, svc, gatherer, fs)
	tmo := httpserver.ServerTimeouts{
		ReadHeader: durationSecEnv("HTTP_READ_HEADER_TIMEOUT_SEC", 5),
		Read:       durationSecEnv("HTTP_READ_TIMEOUT_SEC", 120),
		Write:      durationSecEnv("HTTP_WRITE_TIMEOUT_SEC", 120),
		Idle:       durationSecEnv("HTTP_IDLE_TIMEOUT_SEC", 60),
	}

	var unixPath string
	if p, ok := parseUnixListen(listen); ok {
		unixPath = p
		if err := os.RemoveAll(unixPath); err != nil && !os.IsNotExist(err) {
			log.Error("socket unix", "path", unixPath, "err", err)
			os.Exit(1)
		}
		ln, err := net.Listen("unix", unixPath)
		if err != nil {
			log.Error("listen unix", "path", unixPath, "err", err)
			os.Exit(1)
		}
		if err := os.Chmod(unixPath, 0o666); err != nil {
			log.Error("chmod socket", "path", unixPath, "err", err)
			os.Exit(1)
		}
		srv := httpserver.NewServer("", h, tmo)
		go func() {
			log.Info("http escutando", "addr", listen)
			if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
				log.Error("servidor", "err", err)
				os.Exit(1)
			}
		}()
		waitShutdown(srv, log, unixPath)
		return
	}

	srv := httpserver.NewServer(listen, h, tmo)
	go func() {
		log.Info("http escutando", "addr", listen)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("servidor", "err", err)
			os.Exit(1)
		}
	}()
	waitShutdown(srv, log, "")
}

func parseUnixListen(addr string) (string, bool) {
	a := strings.TrimSpace(addr)
	if len(a) < 6 || !strings.EqualFold(a[:5], "unix:") {
		return "", false
	}
	path := filepath.Clean(strings.TrimSpace(a[5:]))
	if path == "" || path == "." {
		return "", false
	}
	return path, true
}

func waitShutdown(srv *http.Server, log *slog.Logger, unixPath string) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Warn("shutdown http", "err", err)
	}
	if unixPath != "" {
		if err := os.Remove(unixPath); err != nil && !os.IsNotExist(err) {
			log.Warn("remover socket unix", "path", unixPath, "err", err)
		}
	}
}

func ivfPathFor(refPath, explicit string) string {
	if explicit != "" {
		return explicit
	}
	lower := strings.ToLower(refPath)
	if i := strings.LastIndex(lower, ".rbin"); i >= 0 {
		return refPath[:i] + ".ivf"
	}
	return refPath + ".ivf"
}

func pickReferencePath(refPath string, log *slog.Logger) string {
	if st, err := os.Stat(refPath); err == nil && !st.IsDir() {
		return refPath
	}
	if os.Getenv("ALLOW_SMALL_REFERENCES") == "1" {
		if st, err := os.Stat(embeddedReferencesRBin); err == nil && !st.IsDir() {
			log.Warn(
				"referências de desenvolvimento embutidas",
				"requested", refPath,
				"fallback", embeddedReferencesRBin,
			)
			return embeddedReferencesRBin
		}
	}
	return refPath
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
