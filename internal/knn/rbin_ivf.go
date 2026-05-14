package knn

import (
	"encoding/binary"
	"math"
	"sort"

	"heimdall/internal/reference"
	"heimdall/internal/vector"
)

func dist2QueryCentroid(q *[reference.VectorDim]float64, cents []float32, c int) float64 {
	off := c * reference.VectorDim
	var s float64
	for d := 0; d < reference.VectorDim; d++ {
		df := q[d] - float64(cents[off+d])
		s += df * df
	}
	return s
}

func FraudFractionRBinIVF(q *[reference.VectorDim]float64, data []byte, n int, ivf *reference.MappedIVF, nprobe int, maxCand int) float64 {
	if ivf == nil || nprobe < 1 {
		return FraudFractionRBin(q, data, n)
	}
	if n == 0 {
		return 0
	}
	k := kNeighbors
	if n < k {
		k = n
	}

	nList := ivf.NList()
	if nprobe > nList {
		nprobe = nList
	}
	cents := ivf.Centroids()
	offs := ivf.PostingOffsets()
	posts := ivf.Postings()

	type cd struct {
		idx int
		d2  float64
	}
	cds := make([]cd, nList)
	for c := 0; c < nList; c++ {
		cds[c] = cd{idx: c, d2: dist2QueryCentroid(q, cents, c)}
	}
	sort.Slice(cds, func(i, j int) bool {
		if cds[i].d2 != cds[j].d2 {
			return cds[i].d2 < cds[j].d2
		}
		return cds[i].idx < cds[j].idx
	})

	totalCand := 0
	for i := 0; i < nprobe; i++ {
		ci := cds[i].idx
		totalCand += int(offs[ci+1] - offs[ci])
	}
	if totalCand > maxCand || totalCand < k {
		return FraudFractionRBin(q, data, n)
	}

	body := data[reference.RbinBodyOffset(data):]
	stride := reference.RbinRowStride
	ver := binary.LittleEndian.Uint32(data[4:8])
	usePart := ver >= reference.RbinVersion2
	var qPart uint8
	if usePart {
		qPart = vector.PartitionKey(q)
	}
	local := make([]rbinCand, k)
	for i := range local {
		local[i].d2 = math.MaxFloat64
	}

	for i := 0; i < nprobe; i++ {
		ci := cds[i].idx
		lo, hi := offs[ci], offs[ci+1]
		for p := lo; p < hi; p++ {
			ri := int(posts[p])
			if ri < 0 || ri >= n {
				return FraudFractionRBin(q, data, n)
			}
			if usePart && body[ri*stride+57] != qPart {
				continue
			}
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
	return fraudFractionFromCandidates(local, k)
}
