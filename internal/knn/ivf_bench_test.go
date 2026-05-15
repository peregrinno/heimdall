package knn

import (
	"encoding/binary"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"heimdall/internal/reference"
	"heimdall/internal/vector"
)

func BenchmarkFraudFractionRBinIVF(b *testing.B) {
	const n = 100_000
	const nList = 64

	dir := b.TempDir()
	rbinPath := filepath.Join(dir, "refs.rbin")
	ivfPath := filepath.Join(dir, "refs.ivf")

	data := makeSyntheticRBin(b, n)
	if err := os.WriteFile(rbinPath, data, 0o644); err != nil {
		b.Fatal(err)
	}
	_, nLO, cents, offs, posts, err := reference.TrainIVFFromRBin(rbinPath, nList, 8, 11)
	if err != nil {
		b.Fatal(err)
	}
	if err := reference.WriteIVFFile(ivfPath, n, nLO, cents, offs, posts); err != nil {
		b.Fatal(err)
	}

	m, err := reference.OpenMappedRBin(rbinPath)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = m.Close() }()
	ivf, err := reference.OpenMappedIVF(ivfPath)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = ivf.Close() }()

	queries := makeBenchQueries(b, 256)
	const nprobe = 12
	const maxCand = 4_000

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q := &queries[i&255]
		_ = FraudFractionRBinIVF(q, m.Raw(), m.Len(), ivf, nprobe, maxCand)
	}
}

func makeBenchQueries(tb testing.TB, n int) [][reference.VectorDim]float64 {
	tb.Helper()
	rng := rand.New(rand.NewSource(2026))
	qs := make([][reference.VectorDim]float64, n)
	for i := range qs {
		for j := 0; j < reference.VectorDim; j++ {
			qs[i][j] = rng.NormFloat64()
		}
	}
	return qs
}

var _ = math.Float32bits
var _ = binary.LittleEndian
var _ = vector.PartitionKey
