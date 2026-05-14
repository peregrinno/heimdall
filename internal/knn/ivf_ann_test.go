package knn

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"heimdall/internal/reference"
)

func TestFraudFractionRBinIVF_AllListsMatchesExact(t *testing.T) {
	dir := t.TempDir()
	rbinPath := filepath.Join(dir, "x.rbin")
	ivfPath := filepath.Join(dir, "x.ivf")

	const n = 80
	hdr := make([]byte, reference.RbinHeaderSize)
	hdr[0], hdr[1], hdr[2], hdr[3] = 'R', 'R', 'E', 'F'
	binary.LittleEndian.PutUint32(hdr[4:8], 1)
	binary.LittleEndian.PutUint32(hdr[8:12], uint32(n))
	binary.LittleEndian.PutUint16(hdr[12:14], 14)
	body := make([]byte, n*reference.RbinRowStride)
	for i := 0; i < n; i++ {
		row := body[i*reference.RbinRowStride : (i+1)*reference.RbinRowStride]
		for j := 0; j < reference.VectorDim; j++ {
			binary.LittleEndian.PutUint32(row[j*4:j*4+4], math.Float32bits(float32(i%13)+float32(j)*0.01))
		}
		if i%11 == 0 {
			row[56] = 1
		}
	}
	if err := os.WriteFile(rbinPath, append(hdr, body...), 0o644); err != nil {
		t.Fatal(err)
	}

	const nList = 8
	n2, nLO, cents, offs, posts, err := reference.TrainIVFFromRBin(rbinPath, nList, 12, 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := reference.WriteIVFFile(ivfPath, n2, nLO, cents, offs, posts); err != nil {
		t.Fatal(err)
	}

	m, err := reference.OpenMappedRBin(rbinPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = m.Close() }()
	ivf, err := reference.OpenMappedIVF(ivfPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ivf.Close() }()

	var q [reference.VectorDim]float64
	for i := range q {
		q[i] = float64(i) * 0.03
	}

	want := FraudFractionRBin(&q, m.Raw(), m.Len())
	got := FraudFractionRBinIVF(&q, m.Raw(), m.Len(), ivf, nList, 1_000_000)
	if want != got {
		t.Fatalf("want %v got %v", want, got)
	}
}
