package knn

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"heimdall/internal/reference"
	"heimdall/internal/vector"
)

func TestFraudFractionRBinV3ScansAllPartitions(t *testing.T) {
	dir := t.TempDir()
	rbinPath := filepath.Join(dir, "refs.rbin")

	const n = 12
	hdr := make([]byte, reference.RbinHeaderSizeV3)
	hdr[0], hdr[1], hdr[2], hdr[3] = 'R', 'R', 'E', 'F'
	binary.LittleEndian.PutUint32(hdr[4:8], reference.RbinVersion3)
	binary.LittleEndian.PutUint32(hdr[8:12], uint32(n))
	binary.LittleEndian.PutUint16(hdr[12:14], 14)

	var partCounts [32]int
	body := make([]byte, n*reference.RbinRowStride)
	for i := 0; i < n; i++ {
		row := body[i*reference.RbinRowStride : (i+1)*reference.RbinRowStride]
		for j := 0; j < reference.VectorDim; j++ {
			binary.LittleEndian.PutUint32(row[j*4:j*4+4], math.Float32bits(0))
		}
		row[56] = 0
		var kb [reference.VectorDim]float64
		if i < 6 {
			kb[0] = 100
		} else {
			kb[0] = 0.01
			kb[11] = 1
		}
		kb[5], kb[6] = -1, -1
		pk := vector.PartitionKey(&kb)
		row[57] = pk
		for j := 0; j < reference.VectorDim; j++ {
			binary.LittleEndian.PutUint32(row[j*4:j*4+4], math.Float32bits(float32(kb[j])))
		}
		partCounts[pk]++
	}

	var partStart [33]uint32
	var off uint32
	for i := 0; i < 32; i++ {
		partStart[i] = off
		off += uint32(partCounts[i])
	}
	partStart[32] = off
	for i := 0; i < 33; i++ {
		binary.LittleEndian.PutUint32(hdr[64+i*4:68+i*4], partStart[i])
	}

	ordered := make([][]byte, 0, n)
	for pk := 0; pk < 32; pk++ {
		for i := 0; i < n; i++ {
			row := body[i*reference.RbinRowStride : (i+1)*reference.RbinRowStride]
			if row[57] == uint8(pk) {
				ordered = append(ordered, append([]byte(nil), row...))
			}
		}
	}
	if len(ordered) != n {
		t.Fatalf("ordered rows=%d want %d", len(ordered), n)
	}

	var fileBody []byte
	for _, row := range ordered {
		fileBody = append(fileBody, row...)
	}
	if err := os.WriteFile(rbinPath, append(hdr, fileBody...), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := reference.OpenMappedRBin(rbinPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = m.Close() }()

	var q [reference.VectorDim]float64
	q[5], q[6] = -1, -1

	got := FraudFractionRBin(&q, m.Raw(), m.Len())
	want := fraudFractionBruteRBin(&q, m.Raw(), m.Len())
	if got != want {
		t.Fatalf("partition-limited scan would differ: got %v want %v", got, want)
	}
	if got != 0 {
		t.Fatalf("expected 0 fraud fraction, got %v", got)
	}
}
