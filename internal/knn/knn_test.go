package knn

import (
	"testing"

	"heimdall/internal/reference"
)

func TestFraudFraction_AllLegit(t *testing.T) {
	q := &[reference.VectorDim]float64{0, 0, 0, 0, 0, -1, -1, 0, 0, 0, 0, 0, 0.5, 0}
	refs := []reference.Record{
		{Vector: [reference.VectorDim]float64{0.01, 0, 0, 0, 0, -1, -1, 0, 0, 0, 0, 0, 0.5, 0}, Fraud: false},
		{Vector: [reference.VectorDim]float64{0.02, 0, 0, 0, 0, -1, -1, 0, 0, 0, 0, 0, 0.5, 0}, Fraud: false},
		{Vector: [reference.VectorDim]float64{0.03, 0, 0, 0, 0, -1, -1, 0, 0, 0, 0, 0, 0.5, 0}, Fraud: false},
		{Vector: [reference.VectorDim]float64{0.04, 0, 0, 0, 0, -1, -1, 0, 0, 0, 0, 0, 0.5, 0}, Fraud: false},
		{Vector: [reference.VectorDim]float64{0.05, 0, 0, 0, 0, -1, -1, 0, 0, 0, 0, 0, 0.5, 0}, Fraud: false},
		{Vector: [reference.VectorDim]float64{99, 0, 0, 0, 0, -1, -1, 0, 0, 0, 0, 0, 0.5, 0}, Fraud: true},
	}
	if got := FraudFraction(q, refs); got != 0 {
		t.Fatalf("got %v want 0", got)
	}
}

func TestFraudFraction_Mixed(t *testing.T) {
	q := &[reference.VectorDim]float64{}
	refs := make([]reference.Record, 5)
	for i := range refs {
		refs[i].Fraud = i%2 == 0
	}
	if got := FraudFraction(q, refs); got != 0.6 {
		t.Fatalf("got %v want 0.6", got)
	}
}
