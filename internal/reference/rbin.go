package reference

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"

	mmap "github.com/edsrzf/mmap-go"
)

const (
	rbinMagic0 = 'R'
	rbinMagic1 = 'R'
	rbinMagic2 = 'E'
	rbinMagic3 = 'F'
)

const rbinVersion = uint32(1)

const (
	RbinHeaderSize = 64
	RbinRowStride  = 64
	rbinDim        = 14
)

var ErrInvalidRBin = errors.New("references: arquivo .rbin inválido")

type MappedRBin struct {
	data mmap.MMap
	n    int
}

func OpenMappedRBin(path string) (*MappedRBin, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	mm, err := mmap.Map(f, mmap.RDONLY, 0)
	if err != nil {
		return nil, err
	}
	m := &MappedRBin{data: mm}
	if err := m.validate(); err != nil {
		_ = mm.Unmap()
		return nil, err
	}
	return m, nil
}

func (m *MappedRBin) validate() error {
	if len(m.data) < RbinHeaderSize {
		return fmt.Errorf("%w: curto demais", ErrInvalidRBin)
	}
	if m.data[0] != rbinMagic0 || m.data[1] != rbinMagic1 || m.data[2] != rbinMagic2 || m.data[3] != rbinMagic3 {
		return fmt.Errorf("%w: magic", ErrInvalidRBin)
	}
	if binary.LittleEndian.Uint32(m.data[4:8]) != rbinVersion {
		return fmt.Errorf("%w: versão", ErrInvalidRBin)
	}
	dim := binary.LittleEndian.Uint16(m.data[12:14])
	if dim != rbinDim {
		return fmt.Errorf("%w: dim=%d", ErrInvalidRBin, dim)
	}
	n := int(binary.LittleEndian.Uint32(m.data[8:12]))
	if n < 0 {
		return ErrInvalidRBin
	}
	want := RbinHeaderSize + n*RbinRowStride
	if len(m.data) < want {
		return fmt.Errorf("%w: tamanho %d < esperado %d", ErrInvalidRBin, len(m.data), want)
	}
	m.n = n
	return nil
}

func (m *MappedRBin) Close() error {
	if m == nil || m.data == nil {
		return nil
	}
	err := m.data.Unmap()
	m.data = nil
	m.n = 0
	return err
}

func (m *MappedRBin) Len() int { return m.n }

func (m *MappedRBin) Raw() []byte { return []byte(m.data) }
