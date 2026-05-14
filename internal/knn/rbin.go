package knn

import (
	"encoding/binary"
	"math"
	"os"
	"runtime"
	"sort"
	"strconv"
	"sync"

	"heimdall/internal/reference"
)

type rbinCand struct {
	d2    float64
	fraud bool
}

func rowDist2RBin(q *[reference.VectorDim]float64, body []byte, stride, i int) (float64, bool) {
	row := body[i*stride : i*stride+stride]
	var d2 float64
	for j := 0; j < reference.VectorDim; j++ {
		bits := binary.LittleEndian.Uint32(row[j*4 : j*4+4])
		diff := float64(math.Float32frombits(bits)) - q[j]
		d2 += diff * diff
	}
	return d2, row[56] != 0
}

func topKInRangeRBin(q *[reference.VectorDim]float64, body []byte, stride, start, end, k int) []rbinCand {
	if start >= end {
		return nil
	}
	nrows := end - start
	localK := k
	if nrows < localK {
		localK = nrows
	}
	neighbors := make([]rbinCand, localK)
	for i := range neighbors {
		neighbors[i].d2 = math.MaxFloat64
	}
	for i := start; i < end; i++ {
		d2, fraud := rowDist2RBin(q, body, stride, i)
		worst := 0
		for j := 1; j < localK; j++ {
			if neighbors[j].d2 > neighbors[worst].d2 {
				worst = j
			}
		}
		if d2 < neighbors[worst].d2 {
			neighbors[worst] = rbinCand{d2: d2, fraud: fraud}
		}
	}
	return neighbors
}

func fraudFractionFromCandidates(c []rbinCand, k int) float64 {
	if len(c) == 0 {
		return 0
	}
	sort.Slice(c, func(i, j int) bool { return c[i].d2 < c[j].d2 })
	if len(c) < k {
		k = len(c)
	}
	var frauds int
	for i := 0; i < k; i++ {
		if c[i].fraud {
			frauds++
		}
	}
	return float64(frauds) / float64(k)
}

func knnWorkers() int {
	if s := os.Getenv("KNN_WORKERS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	w := runtime.GOMAXPROCS(0)
	if w < 2 {
		return 0
	}
	if w > 16 {
		w = 16
	}
	return w
}

// FraudFractionRBin calcula fração de fraudes entre os k vizinhos mais próximos
// (distância euclidiana; vetores float32 no layout references.rbin).
// Com vários núcleos, usa partição + merge exato: o top-k global está contido na união dos top-k locais.
func FraudFractionRBin(q *[reference.VectorDim]float64, data []byte, n int) float64 {
	if n == 0 {
		return 0
	}
	k := kNeighbors
	if n < k {
		k = n
	}

	body := data[reference.RbinHeaderSize:]
	stride := reference.RbinRowStride

	w := knnWorkers()
	// Partição só vale a pena com volume e paralelismo real (evita overhead em CPU única).
	if w < 2 || n < 50_000 {
		part := topKInRangeRBin(q, body, stride, 0, n, k)
		return fraudFractionFromCandidates(part, k)
	}

	per := (n + w - 1) / w
	partials := make([][]rbinCand, w)
	var wg sync.WaitGroup
	for wi := 0; wi < w; wi++ {
		start := wi * per
		if start >= n {
			break
		}
		end := start + per
		if end > n {
			end = n
		}
		wg.Add(1)
		go func(wi, start, end int) {
			defer wg.Done()
			partials[wi] = topKInRangeRBin(q, body, stride, start, end, kNeighbors)
		}(wi, start, end)
	}
	wg.Wait()

	var all []rbinCand
	for _, p := range partials {
		if len(p) > 0 {
			all = append(all, p...)
		}
	}
	return fraudFractionFromCandidates(all, k)
}
