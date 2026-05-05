package extendible

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

const (
	startDepth = 1

	metaMagic      uint64 = 0x4558544d45544131 // "EXTMETA1"
	metaVersion    uint64 = 1
	metaHeaderSize        = 6 * 8 // magic, version, globalDepth, bucketLimit, nextBucketID, dirCount
)

type Meta struct {
	GlobalDepth  uint64
	BucketLimit  uint64
	NextBucketID uint64
	Directory    []uint64
}

func NewMeta(bucketLimit uint64) *Meta {
	size := uint64(1 << startDepth)
	return &Meta{
		GlobalDepth:  startDepth,
		BucketLimit:  bucketLimit,
		NextBucketID: 3,
		Directory:    make([]uint64, size),
	}
}

func metaFilePath(baseDir string) string {
	return filepath.Join(baseDir, "meta.dat")
}

func metaSerializedSize(dirCount int) int {
	return metaHeaderSize + dirCount*8
}

func loadMetaFromBytes(data []byte) (*Meta, error) {
	if len(data) < metaHeaderSize {
		return nil, fmt.Errorf("meta file too small")
	}

	magic := binary.LittleEndian.Uint64(data[0:8])
	version := binary.LittleEndian.Uint64(data[8:16])
	if magic != metaMagic {
		return nil, fmt.Errorf("invalid meta magic")
	}
	if version != metaVersion {
		return nil, fmt.Errorf("unsupported meta version: %d", version)
	}

	globalDepth := binary.LittleEndian.Uint64(data[16:24])
	bucketLimit := binary.LittleEndian.Uint64(data[24:32])
	nextBucketID := binary.LittleEndian.Uint64(data[32:40])
	dirCount := binary.LittleEndian.Uint64(data[40:48])

	wantSize := metaSerializedSize(int(dirCount))
	if len(data) != wantSize {
		return nil, fmt.Errorf("invalid meta size: got=%d want=%d", len(data), wantSize)
	}
	if dirCount == 0 {
		return nil, fmt.Errorf("empty directory in meta")
	}

	dir := make([]uint64, dirCount)
	offset := metaHeaderSize
	for i := uint64(0); i < dirCount; i++ {
		dir[i] = binary.LittleEndian.Uint64(data[offset : offset+8])
		offset += 8
	}

	return &Meta{
		GlobalDepth:  globalDepth,
		BucketLimit:  bucketLimit,
		NextBucketID: nextBucketID,
		Directory:    dir,
	}, nil
}

func LoadMeta(baseDir string) (*Meta, error) {
	path := metaFilePath(baseDir)
	f, data, err := mmapFileRead(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = closeMappedFile(f, data, false)
	}()

	return loadMetaFromBytes(data)
}

func (m *Meta) serializeInto(data []byte) error {
	if len(m.Directory) == 0 {
		return fmt.Errorf("cannot serialize empty directory")
	}

	wantSize := metaSerializedSize(len(m.Directory))
	if len(data) != wantSize {
		return fmt.Errorf("invalid meta buffer size: got=%d want=%d", len(data), wantSize)
	}

	binary.LittleEndian.PutUint64(data[0:8], metaMagic)
	binary.LittleEndian.PutUint64(data[8:16], metaVersion)
	binary.LittleEndian.PutUint64(data[16:24], m.GlobalDepth)
	binary.LittleEndian.PutUint64(data[24:32], m.BucketLimit)
	binary.LittleEndian.PutUint64(data[32:40], m.NextBucketID)
	binary.LittleEndian.PutUint64(data[40:48], uint64(len(m.Directory)))

	offset := metaHeaderSize
	for _, bucketID := range m.Directory {
		binary.LittleEndian.PutUint64(data[offset:offset+8], bucketID)
		offset += 8
	}
	return nil
}

func (m *Meta) Save(baseDir string) error {
	if len(m.Directory) == 0 {
		return fmt.Errorf("cannot save empty directory")
	}

	path := metaFilePath(baseDir)
	size := metaSerializedSize(len(m.Directory))
	f, data, err := mmapFileWrite(path, size)
	if err != nil {
		return err
	}
	defer func() {
		_ = closeMappedFile(f, data, true)
	}()

	return m.serializeInto(data)
}

func (m *Meta) DirectorySize() uint64 {
	return uint64(len(m.Directory))
}

func (m *Meta) GetBucketID(idx uint64) uint64 {
	if idx >= uint64(len(m.Directory)) {
		return 0
	}
	return m.Directory[idx]
}

func (m *Meta) SetBucketID(idx uint64, bucketID uint64) {
	if idx >= uint64(len(m.Directory)) {
		return
	}
	m.Directory[idx] = bucketID
}

func (m *Meta) DoubleDirectory() {
	oldSize := len(m.Directory)
	newDirectory := make([]uint64, oldSize*2)
	copy(newDirectory[:oldSize], m.Directory)
	copy(newDirectory[oldSize:], m.Directory)
	m.Directory = newDirectory
	m.GlobalDepth++
}

func (m *Meta) RepointAfterSplit(oldBucketID, newBucketID uint64, oldLocalDepth uint64) {
	for idx := uint64(0); idx < uint64(len(m.Directory)); idx++ {
		if m.Directory[idx] != oldBucketID {
			continue
		}
		if ((idx >> oldLocalDepth) & 1) == 1 {
			m.Directory[idx] = newBucketID
		}
	}
}

func (m *Meta) CanShrink() bool {
	if m.GlobalDepth <= startDepth || len(m.Directory)%2 != 0 {
		return false
	}

	half := len(m.Directory) / 2
	for i := 0; i < half; i++ {
		if m.Directory[i] != m.Directory[i+half] {
			return false
		}
	}
	return true
}

func (m *Meta) ShrinkDirectory() {
	for m.CanShrink() {
		half := len(m.Directory) / 2
		next := make([]uint64, half)
		copy(next, m.Directory[:half])
		m.Directory = next
		m.GlobalDepth--
	}
}

func EnsureBaseDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func (m *Meta) Index(hash uint64) uint64 {
	mask := uint64((1 << m.GlobalDepth) - 1)
	return hash & mask
}
