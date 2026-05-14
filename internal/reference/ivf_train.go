package reference

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"os"
	"unsafe"
)

func TrainIVFFromRBin(rbinPath string, nList int, maxIter int, seed int64) (n int, nListOut int, centroids []float32, postingOffsets []uint32, postings []uint32, err error) {
	mm, err := os.ReadFile(rbinPath)
	if err != nil {
		return 0, 0, nil, nil, nil, err
	}
	if len(mm) < RbinHeaderSize {
		return 0, 0, nil, nil, nil, fmt.Errorf("rbin curto")
	}
	ver := binary.LittleEndian.Uint32(mm[4:8])
	if ver != RbinVersion1 && ver != RbinVersion2 && ver != RbinVersion3 {
		return 0, 0, nil, nil, nil, fmt.Errorf("rbin versão %d", ver)
	}
	n = int(binary.LittleEndian.Uint32(mm[8:12]))
	if n < 1 || nList < 1 {
		return 0, 0, nil, nil, nil, fmt.Errorf("n ou nList inválido")
	}
	if nList > n {
		nList = n
	}
	nListOut = nList
	body := mm[RbinBodyOffset(mm):]
	stride := RbinRowStride
	want := n * stride
	if len(body) < want {
		return 0, 0, nil, nil, nil, fmt.Errorf("rbin body curto")
	}

	rng := rand.New(rand.NewSource(seed))
	centroids = make([]float32, nList*VectorDim)
	for j := 0; j < nList; j++ {
		idx := j * n / nList
		if idx >= n {
			idx = n - 1
		}
		row := body[idx*stride : idx*stride+stride]
		v := (*[VectorDim]float32)(unsafe.Pointer(&row[0]))
		copy(centroids[j*VectorDim:(j+1)*VectorDim], v[:])
	}

	assign := make([]uint16, n)
	sum := make([]float64, nList*VectorDim)
	counts := make([]int, nList)

	for iter := 0; iter < maxIter; iter++ {
		clear(sum)
		clear(counts)
		changed := false
		for i := 0; i < n; i++ {
			row := body[i*stride : i*stride+stride]
			c := nearestCentroid(row, centroids, nList)
			if int(assign[i]) != c {
				changed = true
			}
			assign[i] = uint16(c)
			counts[c]++
			v := (*[VectorDim]float32)(unsafe.Pointer(&row[0]))
			off := c * VectorDim
			for d := 0; d < VectorDim; d++ {
				sum[off+d] += float64(v[d])
			}
		}
		for c := 0; c < nList; c++ {
			if counts[c] == 0 {
				// ressincroniza cluster vazio com linha aleatória
				ri := rng.Intn(n)
				row := body[ri*stride : ri*stride+stride]
				v := (*[VectorDim]float32)(unsafe.Pointer(&row[0]))
				copy(centroids[c*VectorDim:(c+1)*VectorDim], v[:])
				continue
			}
			off := c * VectorDim
			inv := 1.0 / float64(counts[c])
			for d := 0; d < VectorDim; d++ {
				centroids[off+d] = float32(sum[off+d] * inv)
			}
		}
		if !changed && iter > 0 {
			break
		}
	}

	postingOffsets = make([]uint32, nList+1)
	pcount := make([]uint32, nList)
	for _, c := range assign {
		pcount[c]++
	}
	for c := 0; c < nList; c++ {
		postingOffsets[c+1] = postingOffsets[c] + pcount[c]
	}
	postings = make([]uint32, n)
	head := append([]uint32(nil), postingOffsets[:nList]...)
	for i, c := range assign {
		pos := head[c]
		head[c]++
		postings[pos] = uint32(i)
	}
	for c := 0; c < nList; c++ {
		if head[c] != postingOffsets[c+1] {
			return 0, 0, nil, nil, nil, fmt.Errorf("ivf interno: cluster %d head %d fim %d", c, head[c], postingOffsets[c+1])
		}
	}
	return n, nListOut, centroids, postingOffsets, postings, nil
}

func nearestCentroid(row []byte, centroids []float32, nList int) int {
	v := (*[VectorDim]float32)(unsafe.Pointer(&row[0]))
	best := 0
	bestD := math.MaxFloat64
	for c := 0; c < nList; c++ {
		off := c * VectorDim
		var s float64
		for d := 0; d < VectorDim; d++ {
			df := float64(v[d]) - float64(centroids[off+d])
			s += df * df
		}
		if s < bestD {
			bestD, best = s, c
		}
	}
	return best
}
