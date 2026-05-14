package knn

import (
	"os"
	"testing"

	"heimdall/internal/reference"
)

func BenchmarkFraudFractionRBin_500k(b *testing.B) {
	benchmarkFraudFractionRBin(b, 500_000, "1")
}

func BenchmarkFraudFractionRBin_500k_workers8(b *testing.B) {
	benchmarkFraudFractionRBin(b, 500_000, "8")
}

func BenchmarkFraudFractionRBin_3M_workers8(b *testing.B) {
	if os.Getenv("HEIMDALL_BENCH_HEAVY") == "" {
		b.Skip("defina HEIMDALL_BENCH_HEAVY=1 para rodar (~200MB+ de RAM por iteração)")
	}
	benchmarkFraudFractionRBin(b, 3_000_000, "8")
}

func benchmarkFraudFractionRBin(b *testing.B, n int, workers string) {
	b.Helper()
	data := makeSyntheticRBin(b, n)
	var q [reference.VectorDim]float64
	for i := range q {
		q[i] = float64(i%7) * 0.01
	}
	b.Setenv("KNN_WORKERS", workers)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FraudFractionRBin(&q, data, n)
	}
}
