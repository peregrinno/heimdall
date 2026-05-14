package knn

import (
	"encoding/binary"
	"math"
	"os"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"unsafe"

	"heimdall/internal/reference"
	"heimdall/internal/vector"
)

type rbinCand struct {
	d2    float64
	fraud bool
}

func rowDist2RBin(q *[reference.VectorDim]float64, body []byte, stride, i int) (float64, bool) {
	row := body[i*stride : i*stride+stride]
	v := (*[reference.VectorDim]float32)(unsafe.Pointer(&row[0]))
	d0 := float64(v[0]) - q[0]
	d1 := float64(v[1]) - q[1]
	d2 := float64(v[2]) - q[2]
	d3 := float64(v[3]) - q[3]
	d4 := float64(v[4]) - q[4]
	d5 := float64(v[5]) - q[5]
	d6 := float64(v[6]) - q[6]
	d7 := float64(v[7]) - q[7]
	d8 := float64(v[8]) - q[8]
	d9 := float64(v[9]) - q[9]
	d10 := float64(v[10]) - q[10]
	d11 := float64(v[11]) - q[11]
	d12 := float64(v[12]) - q[12]
	d13 := float64(v[13]) - q[13]
	s := d0*d0 + d1*d1 + d2*d2 + d3*d3 + d4*d4 + d5*d5 + d6*d6 + d7*d7 +
		d8*d8 + d9*d9 + d10*d10 + d11*d11 + d12*d12 + d13*d13
	return s, row[56] != 0
}

func topKInRangeRBinInto(dst []rbinCand, q *[reference.VectorDim]float64, body []byte, stride, start, end, k int, usePart bool, qPart uint8) int {
	if start >= end {
		return 0
	}
	nrows := end - start
	localK := k
	if nrows < localK {
		localK = nrows
	}
	neighbors := dst[:localK]
	for i := range neighbors {
		neighbors[i].d2 = math.MaxFloat64
	}
	for i := start; i < end; i++ {
		if usePart && body[i*stride+57] != qPart {
			continue
		}
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
	filled := 0
	for i := 0; i < localK; i++ {
		if neighbors[i].d2 < math.MaxFloat64 {
			neighbors[filled] = neighbors[i]
			filled++
		}
	}
	return filled
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
	if w > 8 {
		w = 8
	}
	return w
}

func knnParallelMinScan() int {
	if s := os.Getenv("KNN_PARALLEL_MIN_SCAN"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
			return n
		}
	}
	return 120_000
}

func FraudFractionRBin(q *[reference.VectorDim]float64, data []byte, n int) float64 {
	if n == 0 {
		return 0
	}
	stride := reference.RbinRowStride
	ver := binary.LittleEndian.Uint32(data[4:8])
	hdrSz := reference.RbinBodyOffset(data)
	body := data[hdrSz:]

	var qPart uint8
	if ver >= reference.RbinVersion2 {
		qPart = vector.PartitionKey(q)
	}

	rowStart, rowEnd := reference.RbinPartitionRowRange(data, ver, qPart)
	if ver != reference.RbinVersion3 {
		rowStart, rowEnd = 0, n
	}
	usePart := ver == reference.RbinVersion2

	scanN := rowEnd - rowStart
	k := kNeighbors
	if scanN < k {
		k = scanN
	}
	if k == 0 {
		return 0
	}

	w := knnWorkers()
	if w < 2 || scanN < knnParallelMinScan() {
		local := make([]rbinCand, k)
		nf := topKInRangeRBinInto(local, q, body, stride, rowStart, rowEnd, k, usePart, qPart)
		return fraudFractionFromCandidates(local[:nf], k)
	}

	per := (scanN + w - 1) / w
	buf := make([]rbinCand, w*k)
	nfills := make([]int, w)
	var wg sync.WaitGroup
	launched := 0
	for wi := 0; wi < w; wi++ {
		start := rowStart + wi*per
		if start >= rowEnd {
			break
		}
		end := start + per
		if end > rowEnd {
			end = rowEnd
		}
		launched++
		wg.Add(1)
		go func(wi, start, end int) {
			defer wg.Done()
			nfills[wi] = topKInRangeRBinInto(buf[wi*k:(wi+1)*k], q, body, stride, start, end, k, usePart, qPart)
		}(wi, start, end)
	}
	wg.Wait()
	all := make([]rbinCand, 0, launched*k)
	for wi := 0; wi < launched; wi++ {
		nf := nfills[wi]
		if nf > 0 {
			all = append(all, buf[wi*k:wi*k+nf]...)
		}
	}
	return fraudFractionFromCandidates(all, k)
}
