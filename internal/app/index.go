package app

import (
	"fmt"
	"os"
	"strings"

	"heimdall/internal/knn"
	"heimdall/internal/reference"
)

type ReferenceIndex interface {
	Len() int
	KNNMode() string
	FraudFraction(q *[reference.VectorDim]float64) float64
	Warmup()
	Close() error
}

type ReferenceIndexConfig struct {
	// KNNMode "auto" (usa .ivf se existir), "ivf" (exato se .ivf ausente), ou "exact".
	KNNMode string
	// IVFPath caminho do .ivf; vazio + modo ivf → mesmo diretório do .rbin com sufixo .ivf
	IVFPath    string
	IVFProbes  int
	IVFMaxCand int
}

type hybridIndex struct {
	mem     []reference.Record
	mmap    *reference.MappedRBin
	ivf     *reference.MappedIVF
	mode    string
	nprobe  int
	maxCand int
}

func OpenReferenceIndex(refPath string, cfg ReferenceIndexConfig) (ReferenceIndex, error) {
	lower := strings.ToLower(refPath)
	if strings.HasSuffix(lower, ".rbin") {
		m, err := reference.OpenMappedRBin(refPath)
		if err != nil {
			return nil, err
		}
		h := &hybridIndex{mmap: m, mode: "exact", nprobe: 16, maxCand: 10_000}
		mode := strings.ToLower(strings.TrimSpace(cfg.KNNMode))
		if mode == "" {
			mode = "exact"
		}
		ivfPath := strings.TrimSpace(cfg.IVFPath)
		if ivfPath == "" {
			if i := strings.LastIndex(lower, ".rbin"); i >= 0 {
				ivfPath = refPath[:i] + ".ivf"
			} else {
				ivfPath = refPath + ".ivf"
			}
		}
		if mode == "auto" || mode == "ivf" {
			ivf, err := openIVFIfPresent(ivfPath)
			if err != nil {
				if mode == "ivf" {
					_ = m.Close()
					return nil, fmt.Errorf("ivf %s: %w", ivfPath, err)
				}
			} else if ivf != nil {
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

func openIVFIfPresent(ivfPath string) (*reference.MappedIVF, error) {
	if ivfPath == "" {
		return nil, nil
	}
	if _, err := os.Stat(ivfPath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return reference.OpenMappedIVF(ivfPath)
}

func (h *hybridIndex) Len() int {
	if h.mmap != nil {
		return h.mmap.Len()
	}
	return len(h.mem)
}

func (h *hybridIndex) KNNMode() string {
	if h.ivf != nil && h.mode == "ivf" {
		return "ivf"
	}
	return "exact"
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

func (h *hybridIndex) Warmup() {
	if h.ivf != nil {
		cents := h.ivf.Centroids()
		var fs float32
		for i := 0; i < len(cents); i++ {
			fs += cents[i]
		}
		offs := h.ivf.PostingOffsets()
		var u uint32
		for i := 0; i < len(offs); i++ {
			u ^= offs[i]
		}
		posts := h.ivf.Postings()
		var p uint32
		for i := 0; i < len(posts); i++ {
			p ^= posts[i]
		}
		_ = fs
		_ = u
		_ = p
	}
	// Pre-touch leve do .rbin: lê 1 byte por página de 4 KiB.
	// Sob limite de memória, isso popula o page cache do kernel sem inflar RSS
	// de forma anônima e elimina o pico inicial de page faults sob carga.
	// Custo: ~50ms para 192 MB; ganho: p99 inicial muito menor.
	if h.mmap != nil {
		raw := h.mmap.Raw()
		const pageSize = 4096
		var acc byte
		for i := 0; i < len(raw); i += pageSize {
			acc ^= raw[i]
		}
		_ = acc
	}
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
