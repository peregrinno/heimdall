package app

import (
	"context"
	"log/slog"
	"sync/atomic"

	"heimdall/internal/config"
	"heimdall/internal/domain"
	"heimdall/internal/vector"
)

type Service struct {
	norm   config.Normalization
	mcc    config.MCCRisk
	idx    ReferenceIndex
	ready  atomic.Bool
	logger *slog.Logger
}

func NewService(logger *slog.Logger, norm config.Normalization, mcc config.MCCRisk, idx ReferenceIndex) *Service {
	s := &Service{norm: norm, mcc: mcc, idx: idx, logger: logger}
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
	score := s.idx.FraudFraction(&v)
	approved := score < 0.6
	return domain.FraudScoreResponse{Approved: approved, FraudScore: score}, nil
}
