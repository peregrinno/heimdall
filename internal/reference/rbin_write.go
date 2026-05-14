package reference

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"

	"heimdall/internal/vector"
)

func BuildRBin(r io.Reader, outPath string) (int, error) {
	dec := json.NewDecoder(r)
	tok, err := dec.Token()
	if err != nil {
		return 0, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '[' {
		return 0, ErrInvalidReferencesJSON
	}

	tmpdir, err := os.MkdirTemp("", "rbinbuild")
	if err != nil {
		return 0, err
	}
	defer func() { _ = os.RemoveAll(tmpdir) }()

	bw := make([]*bufio.Writer, 32)
	bf := make([]*os.File, 32)
	defer func() {
		for i := range bf {
			if bf[i] != nil {
				_ = bf[i].Close()
			}
		}
	}()
	for i := 0; i < 32; i++ {
		f, err := os.Create(filepath.Join(tmpdir, strconv.Itoa(i)))
		if err != nil {
			return 0, err
		}
		bf[i] = f
		bw[i] = bufio.NewWriterSize(f, 1<<16)
	}

	row := make([]byte, RbinRowStride)
	n := 0
	var counts [32]int
	for dec.More() {
		var raw rawRecord
		if err := dec.Decode(&raw); err != nil {
			return 0, err
		}
		if len(raw.Vector) != VectorDim {
			return 0, fmt.Errorf("references: dimensão %d, esperado %d", len(raw.Vector), VectorDim)
		}
		clear(row)
		var kb [VectorDim]float64
		for j, v := range raw.Vector {
			kb[j] = v
			binary.LittleEndian.PutUint32(row[j*4:j*4+4], math.Float32bits(float32(v)))
		}
		pk := vector.PartitionKey(&kb)
		row[57] = pk
		if raw.Label == "fraud" {
			row[56] = 1
		}
		if _, err := bw[pk].Write(row); err != nil {
			return 0, err
		}
		counts[pk]++
		n++
	}
	if _, err := dec.Token(); err != nil {
		return 0, err
	}
	for i := 0; i < 32; i++ {
		if err := bw[i].Flush(); err != nil {
			return 0, err
		}
		if err := bf[i].Close(); err != nil {
			return 0, err
		}
		bf[i] = nil
	}

	partStart := make([]uint32, 33)
	for i := 0; i < 32; i++ {
		partStart[i+1] = partStart[i] + uint32(counts[i])
	}
	if int(partStart[32]) != n {
		return 0, fmt.Errorf("references: contagem de partição inconsistente")
	}

	out, err := os.Create(outPath)
	if err != nil {
		return 0, err
	}
	defer func() { _ = out.Close() }()

	hdr := make([]byte, RbinHeaderSizeV3)
	hdr[0], hdr[1], hdr[2], hdr[3] = rbinMagic0, rbinMagic1, rbinMagic2, rbinMagic3
	binary.LittleEndian.PutUint32(hdr[4:8], RbinVersion3)
	binary.LittleEndian.PutUint32(hdr[8:12], uint32(n))
	binary.LittleEndian.PutUint16(hdr[12:14], rbinDim)
	for i := 0; i < 33; i++ {
		binary.LittleEndian.PutUint32(hdr[64+i*4:68+i*4], partStart[i])
	}
	if _, err := out.Write(hdr); err != nil {
		return 0, err
	}
	for i := 0; i < 32; i++ {
		if counts[i] == 0 {
			continue
		}
		f, err := os.Open(filepath.Join(tmpdir, strconv.Itoa(i)))
		if err != nil {
			return 0, err
		}
		_, err = io.Copy(out, f)
		_ = f.Close()
		if err != nil {
			return 0, err
		}
	}
	return n, nil
}
