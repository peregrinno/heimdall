package app

import (
	"fmt"
	"strings"

	"heimdall/internal/knn"
	"heimdall/internal/reference"
)

type ReferenceIndex interface {
	Len() int
	FraudFraction(q *[reference.VectorDim]float64) float64
	Close() error
}

type ReferenceIndexConfig struct {
	// KNNMode "exact" (padrão) ou "ivf" (ANN + re-ranking exato sobre candidatos).
	KNNMode string
	// IVFPath caminho do .ivf; vazio + modo ivf → mesmo diretório do .rbin com sufixo .ivf
	IVFPath string
	IVFProbes  int
	IVFMaxCand int
}

type hybridIndex struct {
	mem   []reference.Record
	mmap  *reference.MappedRBin
	ivf   *reference.MappedIVF
	mode  string
	nprobe int
	maxCand int
}

func OpenReferenceIndex(refPath string, cfg ReferenceIndexConfig) (ReferenceIndex, error) {
	lower := strings.ToLower(refPath)
	if strings.HasSuffix(lower, ".rbin") {
		m, err := reference.OpenMappedRBin(refPath)
		if err != nil {
			return nil, err
		}
		h := &hybridIndex{mmap: m, mode: "exact", nprobe: 24, maxCand: 300_000}
		mode := strings.ToLower(strings.TrimSpace(cfg.KNNMode))
		if mode == "" {
			mode = "exact"
		}
		if mode == "ivf" {
			ivfPath := strings.TrimSpace(cfg.IVFPath)
			if ivfPath == "" {
				if i := strings.LastIndex(lower, ".rbin"); i >= 0 {
					ivfPath = refPath[:i] + ".ivf"
				} else {
					ivfPath = refPath + ".ivf"
				}
			}
			ivf, err := reference.OpenMappedIVF(ivfPath)
			if err != nil {
				_ = m.Close()
				return nil, fmt.Errorf("ivf %s: %w", ivfPath, err)
			}
			if err := ivf.ValidateN(m.Len()); err != nil {
				_ = ivf.Close()
				_ = m.Close()
				return nil, err
			}
			h.ivf = ivf
			h.mode = "ivf"
			if cfg.IVFProbes > 0 {
				h.nprobe = cfg.IVFProbes
			}
			if cfg.IVFMaxCand > 0 {
				h.maxCand = cfg.IVFMaxCand
			}
		}
		return h, nil
	}
	recs, err := reference.LoadGZIP(refPath)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(strings.TrimSpace(cfg.KNNMode), "ivf") {
		return nil, fmt.Errorf("KNN_MODE=ivf exige índice .rbin mmap")
	}
	return &hybridIndex{mem: recs}, nil
}

func (h *hybridIndex) Len() int {
	if h.mmap != nil {
		return h.mmap.Len()
	}
	return len(h.mem)
}

func (h *hybridIndex) FraudFraction(q *[reference.VectorDim]float64) float64 {
	if h.mmap != nil {
		if h.ivf != nil && h.mode == "ivf" {
			return knn.FraudFractionRBinIVF(q, h.mmap.Raw(), h.mmap.Len(), h.ivf, h.nprobe, h.maxCand)
		}
		return knn.FraudFractionRBin(q, h.mmap.Raw(), h.mmap.Len())
	}
	return knn.FraudFraction(q, h.mem)
}

func (h *hybridIndex) Close() error {
	var err0 error
	if h.ivf != nil {
		if err := h.ivf.Close(); err != nil {
			err0 = err
		}
		h.ivf = nil
	}
	if h.mmap != nil {
		if err := h.mmap.Close(); err != nil && err0 == nil {
			err0 = err
		}
		h.mmap = nil
	}
	return err0
}
