package extendible

import (
	"encoding/binary"
	"fmt"
	"path/filepath"
)

const (
	bucketMagic      uint64 = 0x4558544255434b31 // "EXTBUCK1"
	bucketVersion    uint64 = 1
	bucketHeaderSize        = 5 * 8 // magic, version, id, localDepth, count
	bucketEntrySize         = 16    // int64 key + int64 value
)

type BucketEntry struct {
	Key   int64
	Value int64
}

type Bucket struct {
	id         uint64
	localDepth uint64
	entries    []BucketEntry
}

func NewBucket(id uint64, localDepth uint64) *Bucket {
	return &Bucket{
		id:         id,
		localDepth: localDepth,
		entries:    make([]BucketEntry, 0),
	}
}

func bucketFilePath(baseDir string, id uint64) string {
	return filepath.Join(baseDir, fmt.Sprintf("bucket_%020d.dat", id))
}

func loadBucketFromBytes(id uint64, data []byte) (*Bucket, error) {
	if len(data) < bucketHeaderSize {
		return nil, fmt.Errorf("bucket file too small")
	}

	magic := binary.LittleEndian.Uint64(data[0:8])
	version := binary.LittleEndian.Uint64(data[8:16])
	storedID := binary.LittleEndian.Uint64(data[16:24])
	localDepth := binary.LittleEndian.Uint64(data[24:32])
	count := binary.LittleEndian.Uint64(data[32:40])

	if magic != bucketMagic {
		return nil, fmt.Errorf("invalid bucket magic")
	}
	if version != bucketVersion {
		return nil, fmt.Errorf("unsupported bucket version: %d", version)
	}
	if storedID != id {
		return nil, fmt.Errorf("bucket id mismatch: got %d want %d", storedID, id)
	}

	wantSize := bucketHeaderSize + int(count)*bucketEntrySize
	if len(data) != wantSize {
		return nil, fmt.Errorf("invalid bucket size: got=%d want=%d", len(data), wantSize)
	}

	b := &Bucket{
		id:         id,
		localDepth: localDepth,
		entries:    make([]BucketEntry, int(count)),
	}

	offset := bucketHeaderSize
	for i := 0; i < int(count); i++ {
		b.entries[i] = BucketEntry{
			Key:   int64(binary.LittleEndian.Uint64(data[offset : offset+8])),
			Value: int64(binary.LittleEndian.Uint64(data[offset+8 : offset+16])),
		}
		offset += bucketEntrySize
	}

	return b, nil
}

func LoadBucket(baseDir string, id uint64) (*Bucket, error) {
	path := bucketFilePath(baseDir, id)
	f, data, err := mmapFileRead(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = closeMappedFile(f, data, false)
	}()

	return loadBucketFromBytes(id, data)
}

func bucketSerializedSize(entryCount int) int {
	return bucketHeaderSize + entryCount*bucketEntrySize
}

func (b *Bucket) serializeInto(data []byte) error {
	wantSize := bucketSerializedSize(len(b.entries))
	if len(data) != wantSize {
		return fmt.Errorf("invalid bucket buffer size: got=%d want=%d", len(data), wantSize)
	}

	binary.LittleEndian.PutUint64(data[0:8], bucketMagic)
	binary.LittleEndian.PutUint64(data[8:16], bucketVersion)
	binary.LittleEndian.PutUint64(data[16:24], b.id)
	binary.LittleEndian.PutUint64(data[24:32], b.localDepth)
	binary.LittleEndian.PutUint64(data[32:40], uint64(len(b.entries)))

	offset := bucketHeaderSize
	for _, entry := range b.entries {
		binary.LittleEndian.PutUint64(data[offset:offset+8], uint64(entry.Key))
		binary.LittleEndian.PutUint64(data[offset+8:offset+16], uint64(entry.Value))
		offset += bucketEntrySize
	}

	return nil
}

func DeleteBucketFile(baseDir string, id uint64) error {
	err := osRemove(bucketFilePath(baseDir, id))
	if err == nil || isNotExist(err) {
		return nil
	}
	return err
}

func osRemove(path string) error {
	return removeFile(path)
}

func isNotExist(err error) bool {
	return fileNotExist(err)
}

func (b *Bucket) ID() uint64 {
	return b.id
}

func (b *Bucket) LocalDepth() uint64 {
	return b.localDepth
}

func (b *Bucket) SetLocalDepth(depth uint64) {
	b.localDepth = depth
}

func (b *Bucket) Count() uint64 {
	return uint64(len(b.entries))
}

func (b *Bucket) Get(key int64) (int64, bool) {
	for i := range b.entries {
		if b.entries[i].Key == key {
			return b.entries[i].Value, true
		}
	}
	return 0, false
}

func (b *Bucket) Put(key int64, value int64) (inserted bool, updated bool) {
	for i := range b.entries {
		if b.entries[i].Key == key {
			b.entries[i].Value = value
			return false, true
		}
	}

	b.entries = append(b.entries, BucketEntry{Key: key, Value: value})
	return true, false
}

func (b *Bucket) Delete(key int64) bool {
	for i := range b.entries {
		if b.entries[i].Key != key {
			continue
		}

		last := len(b.entries) - 1
		b.entries[i] = b.entries[last]
		b.entries[last] = BucketEntry{}
		b.entries = b.entries[:last]
		return true
	}

	return false
}

func (b *Bucket) Entries() []BucketEntry {
	cp := make([]BucketEntry, len(b.entries))
	copy(cp, b.entries)
	return cp
}

func (b *Bucket) ReplaceEntries(entries []BucketEntry) {
	b.entries = append(b.entries[:0], entries...)
}

func (b *Bucket) IsFull(limit uint64) bool {
	return uint64(len(b.entries)) >= limit
}

func (b *Bucket) Clear() {
	for i := range b.entries {
		b.entries[i] = BucketEntry{}
	}
	b.entries = b.entries[:0]
}

func (b *Bucket) EntriesCount() int {
	return len(b.entries)
}
