package reference

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
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

	f, err := os.Create(outPath)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()

	w := bufio.NewWriterSize(f, 1<<22)
	hdr := make([]byte, RbinHeaderSize)
	hdr[0], hdr[1], hdr[2], hdr[3] = rbinMagic0, rbinMagic1, rbinMagic2, rbinMagic3
	binary.LittleEndian.PutUint32(hdr[4:8], rbinVersion)
	binary.LittleEndian.PutUint32(hdr[8:12], 0) // preenchido ao final
	binary.LittleEndian.PutUint16(hdr[12:14], rbinDim)
	if _, err := w.Write(hdr); err != nil {
		return 0, err
	}

	row := make([]byte, RbinRowStride)
	n := 0
	for dec.More() {
		var raw rawRecord
		if err := dec.Decode(&raw); err != nil {
			return 0, err
		}
		if len(raw.Vector) != VectorDim {
			return 0, fmt.Errorf("references: dimensão %d, esperado %d", len(raw.Vector), VectorDim)
		}
		clear(row)
		for j, v := range raw.Vector {
			binary.LittleEndian.PutUint32(row[j*4:j*4+4], math.Float32bits(float32(v)))
		}
		if raw.Label == "fraud" {
			row[56] = 1
		}
		if _, err := w.Write(row); err != nil {
			return 0, err
		}
		n++
	}
	if _, err := dec.Token(); err != nil {
		return 0, err
	}
	if err := w.Flush(); err != nil {
		return 0, err
	}
	if _, err := f.Seek(8, io.SeekStart); err != nil {
		return 0, err
	}
	var cnt [4]byte
	binary.LittleEndian.PutUint32(cnt[:], uint32(n))
	if _, err := f.Write(cnt[:]); err != nil {
		return 0, err
	}
	return n, nil
}
