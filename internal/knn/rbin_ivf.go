package knn

import (
	"math"
	"sort"
	"sync"

	"heimdall/internal/reference"
)

const ivfMaxTopProbes = 64

var ivfCentroidPool = sync.Pool{
	New: func() any {
		s := make([]ivfCentroidDist, 0, 1024)
		return &s
	},
}

var ivfListPool = sync.Pool{
	New: func() any {
		s := make([]int, 0, ivfMaxTopProbes)
		return &s
	},
}

func dist2QueryCentroid32(q *[reference.VectorDim]float32, cents []float32, c int) float32 {
	off := c * reference.VectorDim
	row := (*[reference.VectorDim]float32)(cents[off : off+reference.VectorDim : off+reference.VectorDim])
	d0 := q[0] - row[0]
	d1 := q[1] - row[1]
	d2 := q[2] - row[2]
	d3 := q[3] - row[3]
	d4 := q[4] - row[4]
	d5 := q[5] - row[5]
	d6 := q[6] - row[6]
	d7 := q[7] - row[7]
	d8 := q[8] - row[8]
	d9 := q[9] - row[9]
	d10 := q[10] - row[10]
	d11 := q[11] - row[11]
	d12 := q[12] - row[12]
	d13 := q[13] - row[13]
	return d0*d0 + d1*d1 + d2*d2 + d3*d3 + d4*d4 + d5*d5 + d6*d6 + d7*d7 +
		d8*d8 + d9*d9 + d10*d10 + d11*d11 + d12*d12 + d13*d13
}

type ivfCentroidDist struct {
	idx int
	d2  float32
}

func topProbesCentroids(top []ivfCentroidDist, q32 *[reference.VectorDim]float32, cents []float32, nList, want int) []ivfCentroidDist {
	if want > nList {
		want = nList
	}
	if want < 1 {
		want = 1
	}
	top = top[:0]
	for c := 0; c < nList; c++ {
		d := dist2QueryCentroid32(q32, cents, c)
		if len(top) < want {
			top = append(top, ivfCentroidDist{idx: c, d2: d})
			if len(top) == want {
				sortIvfDescByDist(top)
			}
			continue
		}
		if d < top[0].d2 {
			top[0] = ivfCentroidDist{idx: c, d2: d}
			siftDownIvfDesc(top, 0)
		}
	}
	sort.Slice(top, func(i, j int) bool { return top[i].d2 < top[j].d2 })
	return top
}

func sortIvfDescByDist(a []ivfCentroidDist) {
	for i := len(a)/2 - 1; i >= 0; i-- {
		siftDownIvfDesc(a, i)
	}
}

func siftDownIvfDesc(a []ivfCentroidDist, i int) {
	n := len(a)
	for {
		l := 2*i + 1
		if l >= n {
			return
		}
		r := l + 1
		c := l
		if r < n && a[r].d2 > a[l].d2 {
			c = r
		}
		if a[i].d2 >= a[c].d2 {
			return
		}
		a[i], a[c] = a[c], a[i]
		i = c
	}
}

func selectProbeLists(cds []ivfCentroidDist, dst []int) []int {
	dst = dst[:0]
	for i := 0; i < len(cds); i++ {
		dst = append(dst, cds[i].idx)
	}
	return dst
}

func FraudFractionRBinIVF(q *[reference.VectorDim]float64, data []byte, n int, ivf *reference.MappedIVF, nprobe int, maxCand int) float64 {
	if ivf == nil || nprobe < 1 {
		return FraudFractionRBin(q, data, n)
	}
	if n == 0 {
		return 0
	}
	if maxCand < 1 {
		maxCand = 1
	}
	k := kNeighbors
	if n < k {
		k = n
	}

	nList := ivf.NList()
	cents := ivf.Centroids()
	offs := ivf.PostingOffsets()
	posts := ivf.Postings()

	wantProbe := nprobe
	if wantProbe > nList {
		wantProbe = nList
	}
	if wantProbe > ivfMaxTopProbes {
		wantProbe = ivfMaxTopProbes
	}

	var q32 [reference.VectorDim]float32
	for i := 0; i < reference.VectorDim; i++ {
		q32[i] = float32(q[i])
	}

	cdsPtr := ivfCentroidPool.Get().(*[]ivfCentroidDist)
	listsPtr := ivfListPool.Get().(*[]int)
	defer func() {
		*cdsPtr = (*cdsPtr)[:0]
		ivfCentroidPool.Put(cdsPtr)
		*listsPtr = (*listsPtr)[:0]
		ivfListPool.Put(listsPtr)
	}()

	cds := topProbesCentroids(*cdsPtr, &q32, cents, nList, wantProbe)
	*cdsPtr = cds
	lists := selectProbeLists(cds, (*listsPtr)[:0])
	*listsPtr = lists
	if len(lists) == 0 {
		lists = append(lists, cds[0].idx)
	}

	body := data[reference.RbinBodyOffset(data):]
	stride := reference.RbinRowStride

	var local [kNeighbors]rbinCand
	for i := 0; i < k; i++ {
		local[i].d2 = math.MaxFloat64
	}

	scanned := 0
	for _, ci := range lists {
		lo, hi := offs[ci], offs[ci+1]
		for p := lo; p < hi; p++ {
			if scanned >= maxCand {
				break
			}
			ri := int(posts[p])
			if ri < 0 || ri >= n {
				continue
			}
			scanned++
			d2, fraud := rowDist2RBin(q, body, stride, ri)
			worst := 0
			for j := 1; j < k; j++ {
				if local[j].d2 > local[worst].d2 {
					worst = j
				}
			}
			if d2 < local[worst].d2 {
				local[worst] = rbinCand{d2: d2, fraud: fraud}
			}
		}
	}
	filled := 0
	for i := 0; i < k; i++ {
		if local[i].d2 < math.MaxFloat64 {
			filled++
		}
	}
	if filled == 0 {
		return 0
	}
	return fraudFractionFromCandidates(local[:filled], k)
}
