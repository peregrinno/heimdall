package reference

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
)

func WriteIVFFile(path string, n, nList int, centroids []float32, postingOffsets []uint32, postings []uint32) error {
	if len(centroids) != nList*VectorDim {
		return fmt.Errorf("centroides: esperado %d floats, tem %d", nList*VectorDim, len(centroids))
	}
	if len(postingOffsets) != nList+1 {
		return fmt.Errorf("offsets: esperado %d, tem %d", nList+1, len(postingOffsets))
	}
	if int(postingOffsets[nList]) != n || len(postings) != n {
		return fmt.Errorf("postings: esperado %d índices", n)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	hdr := make([]byte, IvfHeaderSize)
	hdr[0], hdr[1], hdr[2], hdr[3] = ivfMagic0, ivfMagic1, ivfMagic2, ivfMagic3
	binary.LittleEndian.PutUint32(hdr[4:8], ivfVersion)
	binary.LittleEndian.PutUint32(hdr[8:12], uint32(n))
	binary.LittleEndian.PutUint16(hdr[12:14], uint16(nList))
	binary.LittleEndian.PutUint16(hdr[14:16], uint16(VectorDim))
	if _, err := f.Write(hdr); err != nil {
		return err
	}
	for _, v := range centroids {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], math.Float32bits(v))
		if _, err := f.Write(b[:]); err != nil {
			return err
		}
	}
	for _, o := range postingOffsets {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], o)
		if _, err := f.Write(b[:]); err != nil {
			return err
		}
	}
	for _, id := range postings {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], id)
		if _, err := f.Write(b[:]); err != nil {
			return err
		}
	}
	return nil
}
