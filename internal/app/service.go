package app

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"heimdall/internal/config"
	"heimdall/internal/domain"
	"heimdall/internal/vector"

	"github.com/prometheus/client_golang/prometheus"
)

type Service struct {
	norm   config.Normalization
	mcc    config.MCCRisk
	idx    ReferenceIndex
	ready  atomic.Bool
	logger *slog.Logger
	knnObs prometheus.Observer
}

func NewService(logger *slog.Logger, norm config.Normalization, mcc config.MCCRisk, idx ReferenceIndex, knnObs prometheus.Observer) *Service {
	if knnObs == nil {
		knnObs = prometheus.ObserverFunc(func(float64) {})
	}
	s := &Service{norm: norm, mcc: mcc, idx: idx, logger: logger, knnObs: knnObs}
	s.ready.Store(idx != nil && idx.Len() > 0)
	return s
}

func (s *Service) Close() error {
	if s.idx == nil {
		return nil
	}
	return s.idx.Close()
}

func (s *Service) Ready() bool {
	return s.ready.Load()
}

func (s *Service) Score(ctx context.Context, req domain.FraudScoreRequest) (domain.FraudScoreResponse, error) {
	_ = ctx
	v := vector.Build(req, s.norm, s.mcc)
	t0 := time.Now()
	score := s.idx.FraudFraction(&v)
	s.knnObs.Observe(time.Since(t0).Seconds())
	approved := score < 0.6
	return domain.FraudScoreResponse{Approved: approved, FraudScore: score}, nil
}
