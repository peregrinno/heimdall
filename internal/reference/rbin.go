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

const (
	RbinVersion1 = uint32(1)
	RbinVersion2 = uint32(2)
	RbinVersion3 = uint32(3)
)

const (
	RbinHeaderSize   = 64
	RbinPrefixWords  = 33
	RbinHeaderSizeV3 = RbinHeaderSize + RbinPrefixWords*4
	RbinRowStride    = 64
	rbinDim          = 14
)

func RbinHeaderByteSize(ver uint32) int {
	if ver == RbinVersion3 {
		return RbinHeaderSizeV3
	}
	return RbinHeaderSize
}

func RbinBodyOffset(data []byte) int {
	if len(data) < 8 {
		return RbinHeaderSize
	}
	return RbinHeaderByteSize(binary.LittleEndian.Uint32(data[4:8]))
}

func RbinPartitionRowRange(data []byte, ver uint32, qPart uint8) (start, end int) {
	n := int(binary.LittleEndian.Uint32(data[8:12]))
	if ver != RbinVersion3 || len(data) < RbinHeaderSizeV3 {
		return 0, n
	}
	base := 64 + int(qPart)*4
	if base+8 > len(data) {
		return 0, n
	}
	start = int(binary.LittleEndian.Uint32(data[base : base+4]))
	end = int(binary.LittleEndian.Uint32(data[base+4 : base+8]))
	if start < 0 || end < start || end > n {
		return 0, n
	}
	return start, end
}

var ErrInvalidRBin = errors.New("references: arquivo .rbin inválido")

type MappedRBin struct {
	data mmap.MMap
	n    int
	Ver  uint32
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
	// Acesso por IVF é aleatório (vetores espalhados em ~192 MB).
	// MADV_RANDOM desliga readahead e reduz pressão no page cache.
	adviseRandom([]byte(mm))
	return m, nil
}

func (m *MappedRBin) validate() error {
	if len(m.data) < RbinHeaderSize {
		return fmt.Errorf("%w: curto demais", ErrInvalidRBin)
	}
	if m.data[0] != rbinMagic0 || m.data[1] != rbinMagic1 || m.data[2] != rbinMagic2 || m.data[3] != rbinMagic3 {
		return fmt.Errorf("%w: magic", ErrInvalidRBin)
	}
	ver := binary.LittleEndian.Uint32(m.data[4:8])
	if ver != RbinVersion1 && ver != RbinVersion2 && ver != RbinVersion3 {
		return fmt.Errorf("%w: versão", ErrInvalidRBin)
	}
	m.Ver = ver
	dim := binary.LittleEndian.Uint16(m.data[12:14])
	if dim != rbinDim {
		return fmt.Errorf("%w: dim=%d", ErrInvalidRBin, dim)
	}
	n := int(binary.LittleEndian.Uint32(m.data[8:12]))
	if n < 0 {
		return ErrInvalidRBin
	}
	hdrLen := RbinHeaderByteSize(ver)
	if len(m.data) < hdrLen {
		return fmt.Errorf("%w: cabeçalho", ErrInvalidRBin)
	}
	if ver == RbinVersion3 {
		if binary.LittleEndian.Uint32(m.data[64:68]) != 0 {
			return fmt.Errorf("%w: tabela de partição", ErrInvalidRBin)
		}
		last := int(binary.LittleEndian.Uint32(m.data[64+32*4 : 64+33*4]))
		if last != n {
			return fmt.Errorf("%w: tabela de partição", ErrInvalidRBin)
		}
		for i := 0; i < 32; i++ {
			a := int(binary.LittleEndian.Uint32(m.data[64+i*4 : 68+i*4]))
			b := int(binary.LittleEndian.Uint32(m.data[68+i*4 : 72+i*4]))
			if a > b || b > n {
				return fmt.Errorf("%w: tabela de partição", ErrInvalidRBin)
			}
		}
	}
	want := hdrLen + n*RbinRowStride
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
	m.Ver = 0
	return err
}

func (m *MappedRBin) Len() int { return m.n }

func (m *MappedRBin) Raw() []byte { return []byte(m.data) }
