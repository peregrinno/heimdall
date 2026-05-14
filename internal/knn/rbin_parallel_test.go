package knn

import (
	"encoding/binary"
	"math"
	"math/rand"
	"runtime"
	"sort"
	"testing"

	"heimdall/internal/reference"
)

func TestFraudFractionRBinParallelMatchesBrute(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("precisa de >=2 CPUs para validar caminho paralelo")
	}
	const n = 55_000
	t.Setenv("KNN_WORKERS", "8")

	data := makeSyntheticRBin(t, n)
	rng := rand.New(rand.NewSource(7))
	var q [reference.VectorDim]float64
	for i := range q {
		q[i] = rng.Float64()
	}

	want := fraudFractionBruteRBin(&q, data, n)
	got := FraudFractionRBin(&q, data, n)
	if math.Abs(want-got) > 1e-9 {
		t.Fatalf("want %v got %v", want, got)
	}
}

func makeSyntheticRBin(tb testing.TB, n int) []byte {
	tb.Helper()
	hdr := make([]byte, reference.RbinHeaderSize)
	hdr[0], hdr[1], hdr[2], hdr[3] = 'R', 'R', 'E', 'F'
	binary.LittleEndian.PutUint32(hdr[4:8], 1)
	binary.LittleEndian.PutUint32(hdr[8:12], uint32(n))
	binary.LittleEndian.PutUint16(hdr[12:14], 14)
	body := make([]byte, n*reference.RbinRowStride)
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < n; i++ {
		row := body[i*reference.RbinRowStride : (i+1)*reference.RbinRowStride]
		for j := 0; j < reference.VectorDim; j++ {
			binary.LittleEndian.PutUint32(row[j*4:j*4+4], math.Float32bits(float32(rng.NormFloat64())))
		}
		if i%7 == 0 {
			row[56] = 1
		}
	}
	return append(hdr, body...)
}

func fraudFractionBruteRBin(q *[reference.VectorDim]float64, data []byte, n int) float64 {
	k := kNeighbors
	if n < k {
		k = n
	}
	type item struct {
		d2    float64
		fraud bool
	}
	all := make([]item, n)
	body := data[reference.RbinHeaderSize:]
	stride := reference.RbinRowStride
	for i := 0; i < n; i++ {
		d2, fraud := rowDist2RBin(q, body, stride, i)
		all[i] = item{d2: d2, fraud: fraud}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].d2 < all[j].d2 })
	var f int
	for i := 0; i < k; i++ {
		if all[i].fraud {
			f++
		}
	}
	return float64(f) / float64(k)
}
