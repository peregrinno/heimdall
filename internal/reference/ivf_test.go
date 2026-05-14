package reference

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestIVFWriteReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	rbin := filepath.Join(dir, "t.rbin")
	ivf := filepath.Join(dir, "t.ivf")

	// rbin mínimo: 20 linhas sintéticas (usa o mesmo helper que knn, mas aqui escrevemos via Train input)
	// Construímos bytes de rbin manualmente como em knn tests.
	n := 20
	hdr := make([]byte, RbinHeaderSize)
	hdr[0], hdr[1], hdr[2], hdr[3] = 'R', 'R', 'E', 'F'
	binary.LittleEndian.PutUint32(hdr[4:8], 1)
	binary.LittleEndian.PutUint32(hdr[8:12], uint32(n))
	binary.LittleEndian.PutUint16(hdr[12:14], 14)
	body := make([]byte, n*RbinRowStride)
	for i := 0; i < n; i++ {
		row := body[i*RbinRowStride : (i+1)*RbinRowStride]
		for j := 0; j < VectorDim; j++ {
			binary.LittleEndian.PutUint32(row[j*4:j*4+4], math.Float32bits(float32(i+j)))
		}
	}
	if err := os.WriteFile(rbin, append(hdr, body...), 0o644); err != nil {
		t.Fatal(err)
	}

	n2, nLO, cents, offs, posts, err := TrainIVFFromRBin(rbin, 5, 10, 99)
	if err != nil {
		t.Fatal(err)
	}
	if n2 != n || nLO != 5 {
		t.Fatalf("n=%d nLO=%d", n2, nLO)
	}
	if err := WriteIVFFile(ivf, n2, nLO, cents, offs, posts); err != nil {
		t.Fatal(err)
	}
	m, err := OpenMappedIVF(ivf)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = m.Close() }()
	if m.N() != n || m.NList() != 5 {
		t.Fatalf("mmap n=%d lists=%d", m.N(), m.NList())
	}
}
