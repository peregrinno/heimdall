package knn

import (
	"math"

	"heimdall/internal/reference"
)

const kNeighbors = 5

func dist2(a, b *[reference.VectorDim]float64) float64 {
	var s float64
	for i := 0; i < reference.VectorDim; i++ {
		d := a[i] - b[i]
		s += d * d
	}
	return s
}

func FraudFraction(query *[reference.VectorDim]float64, refs []reference.Record) float64 {
	if len(refs) == 0 {
		return 0
	}
	k := kNeighbors
	if len(refs) < k {
		k = len(refs)
	}

	type nb struct {
		d2    float64
		fraud bool
	}
	neighbors := make([]nb, k)
	for i := range neighbors {
		neighbors[i].d2 = math.MaxFloat64
	}

	for i := range refs {
		d := dist2(query, &refs[i].Vector)
		worst := 0
		for j := 1; j < k; j++ {
			if neighbors[j].d2 > neighbors[worst].d2 {
				worst = j
			}
		}
		if d < neighbors[worst].d2 {
			neighbors[worst] = nb{d2: d, fraud: refs[i].Fraud}
		}
	}

	var frauds int
	for _, n := range neighbors {
		if n.fraud {
			frauds++
		}
	}
	return float64(frauds) / float64(k)
}
