package reference

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"unsafe"

	mmap "github.com/edsrzf/mmap-go"
)

const (
	ivfMagic0 = 'I'
	ivfMagic1 = 'V'
	ivfMagic2 = 'F'
	ivfMagic3 = 'F'
	// IvfHeaderSize tamanho fixo do cabeçalho .ivf (alinhado com RbinHeaderSize).
	IvfHeaderSize = 64
	ivfVersion    = uint32(1)
)

var ErrInvalidIVF = errors.New("references: arquivo .ivf inválido")

type MappedIVF struct {
	data mmap.MMap
	n    int
	nList int
	// byte offsets dentro de data após validação
	centroidsOff int
	offsetsOff   int
	postingsOff  int
}

func OpenMappedIVF(path string) (*MappedIVF, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	mm, err := mmap.Map(f, mmap.RDONLY, 0)
	if err != nil {
		return nil, err
	}
	m := &MappedIVF{data: mm}
	if err := m.validate(); err != nil {
		_ = mm.Unmap()
		return nil, err
	}
	return m, nil
}

func (m *MappedIVF) validate() error {
	if len(m.data) < IvfHeaderSize {
		return fmt.Errorf("%w: curto demais", ErrInvalidIVF)
	}
	if m.data[0] != ivfMagic0 || m.data[1] != ivfMagic1 || m.data[2] != ivfMagic2 || m.data[3] != ivfMagic3 {
		return fmt.Errorf("%w: magic", ErrInvalidIVF)
	}
	if binary.LittleEndian.Uint32(m.data[4:8]) != ivfVersion {
		return fmt.Errorf("%w: versão", ErrInvalidIVF)
	}
	n := int(binary.LittleEndian.Uint32(m.data[8:12]))
	nList := int(binary.LittleEndian.Uint16(m.data[12:14]))
	dim := int(binary.LittleEndian.Uint16(m.data[14:16]))
	if n < 0 || nList < 1 || dim != VectorDim {
		return fmt.Errorf("%w: n=%d nList=%d dim=%d", ErrInvalidIVF, n, nList, dim)
	}
	m.n = n
	m.nList = nList

	m.centroidsOff = IvfHeaderSize
	m.offsetsOff = m.centroidsOff + nList*VectorDim*4
	m.postingsOff = m.offsetsOff + (nList+1)*4
	want := m.postingsOff + n*4
	if len(m.data) < want {
		return fmt.Errorf("%w: tamanho %d < esperado %d", ErrInvalidIVF, len(m.data), want)
	}
	// offsets prefixo: deve fechar em n
	off := unsafe.Slice((*uint32)(unsafe.Pointer(&m.data[m.offsetsOff])), nList+1)
	if int(off[nList]) != n {
		return fmt.Errorf("%w: postings total %d != n %d", ErrInvalidIVF, off[nList], n)
	}
	return nil
}

func (m *MappedIVF) Close() error {
	if m == nil || m.data == nil {
		return nil
	}
	err := m.data.Unmap()
	m.data = nil
	m.n = 0
	m.nList = 0
	return err
}

func (m *MappedIVF) N() int       { return m.n }
func (m *MappedIVF) NList() int   { return m.nList }

func (m *MappedIVF) Centroids() []float32 {
	n := m.nList * VectorDim
	return unsafe.Slice((*float32)(unsafe.Pointer(&m.data[m.centroidsOff])), n)
}

func (m *MappedIVF) PostingOffsets() []uint32 {
	return unsafe.Slice((*uint32)(unsafe.Pointer(&m.data[m.offsetsOff])), m.nList+1)
}

func (m *MappedIVF) Postings() []uint32 {
	return unsafe.Slice((*uint32)(unsafe.Pointer(&m.data[m.postingsOff])), m.n)
}

func (m *MappedIVF) ValidateN(rbinN int) error {
	if m == nil {
		return fmt.Errorf("%w: nil", ErrInvalidIVF)
	}
	if m.n != rbinN {
		return fmt.Errorf("%w: ivf n=%d rbin n=%d", ErrInvalidIVF, m.n, rbinN)
	}
	return nil
}
