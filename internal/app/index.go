package app

import (
	"strings"

	"heimdall/internal/knn"
	"heimdall/internal/reference"
)

type ReferenceIndex interface {
	Len() int
	FraudFraction(q *[reference.VectorDim]float64) float64
	Close() error
}

type hybridIndex struct {
	mem  []reference.Record
	mmap *reference.MappedRBin
}

func OpenReferenceIndex(path string) (ReferenceIndex, error) {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".rbin") {
		m, err := reference.OpenMappedRBin(path)
		if err != nil {
			return nil, err
		}
		return &hybridIndex{mmap: m}, nil
	}
	recs, err := reference.LoadGZIP(path)
	if err != nil {
		return nil, err
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
		return knn.FraudFractionRBin(q, h.mmap.Raw(), h.mmap.Len())
	}
	return knn.FraudFraction(q, h.mem)
}

func (h *hybridIndex) Close() error {
	if h.mmap != nil {
		return h.mmap.Close()
	}
	return nil
}
