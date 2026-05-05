package extendible

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"os"
)

type HashTable struct {
	meta          *Meta
	baseDir       string
	buckets       map[uint64]*Bucket
	dirtyBuckets  map[uint64]struct{}
	deletedBucket map[uint64]struct{}
	metaDirty     bool
	cache         *mmapCache
}

func NewHashTable(bucketLimit uint64) *HashTable {
	dir, err := os.MkdirTemp("", "extendible-hash-*")
	if err != nil {
		panic(err)
	}

	ht, err := NewHashTableOnDisk(dir, bucketLimit)
	if err != nil {
		panic(err)
	}
	return ht
}

func NewHashTableOnDisk(baseDir string, bucketLimit uint64) (*HashTable, error) {
	if bucketLimit == 0 {
		return nil, fmt.Errorf("bucket limit must be positive")
	}
	if err := EnsureBaseDir(baseDir); err != nil {
		return nil, err
	}

	meta := NewMeta(bucketLimit)
	b1 := NewBucket(1, startDepth)
	b2 := NewBucket(2, startDepth)
	meta.SetBucketID(0, b1.ID())
	meta.SetBucketID(1, b2.ID())

	ht := &HashTable{
		meta:          meta,
		baseDir:       baseDir,
		buckets:       map[uint64]*Bucket{b1.ID(): b1, b2.ID(): b2},
		dirtyBuckets:  make(map[uint64]struct{}, 2),
		deletedBucket: make(map[uint64]struct{}),
		metaDirty:     true,
		cache:         newMmapCache(),
	}

	ht.markBucketDirty(b1.ID())
	ht.markBucketDirty(b2.ID())
	if err := ht.Sync(); err != nil {
		return nil, err
	}

	return ht, nil
}

func OpenHashTable(baseDir string) (*HashTable, error) {
	meta, err := LoadMeta(baseDir)
	if err != nil {
		return nil, err
	}

	return &HashTable{
		meta:          meta,
		baseDir:       baseDir,
		buckets:       make(map[uint64]*Bucket),
		dirtyBuckets:  make(map[uint64]struct{}),
		deletedBucket: make(map[uint64]struct{}),
		cache:         newMmapCache(),
	}, nil
}

func (ht *HashTable) BaseDir() string {
	return ht.baseDir
}

func (ht *HashTable) Close() error {
	if err := ht.Sync(); err != nil {
		return err
	}
	return ht.cache.close()
}

func (ht *HashTable) Sync() error {
	if err := ht.flushDirty(); err != nil {
		return err
	}
	return ht.cache.flushDirty()
}

func hashKey(key int64) uint64 {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(key))

	h := fnv.New64a()
	_, _ = h.Write(buf[:])
	return h.Sum64()
}

func (ht *HashTable) bucketByID(bucketID uint64) (*Bucket, error) {
	if bucket, ok := ht.buckets[bucketID]; ok {
		return bucket, nil
	}

	bucket, err := LoadBucket(ht.baseDir, bucketID)
	if err != nil {
		return nil, err
	}
	ht.buckets[bucketID] = bucket
	return bucket, nil
}

func (ht *HashTable) bucketByKey(key int64) (*Bucket, uint64, uint64, error) {
	hash := hashKey(key)
	idx := ht.meta.Index(hash)
	bucketID := ht.meta.GetBucketID(idx)
	bucket, err := ht.bucketByID(bucketID)
	return bucket, bucketID, idx, err
}

func (ht *HashTable) markBucketDirty(bucketID uint64) {
	delete(ht.deletedBucket, bucketID)
	ht.dirtyBuckets[bucketID] = struct{}{}
}

func (ht *HashTable) markBucketDeleted(bucketID uint64) {
	delete(ht.dirtyBuckets, bucketID)
	ht.deletedBucket[bucketID] = struct{}{}
}

func (ht *HashTable) flushMeta() error {
	path := metaFilePath(ht.baseDir)
	size := metaSerializedSize(len(ht.meta.Directory))
	if err := ht.cache.write(path, size, ht.meta.serializeInto); err != nil {
		return err
	}
	ht.metaDirty = false
	return nil
}

func (ht *HashTable) flushBucket(bucket *Bucket) error {
	path := bucketFilePath(ht.baseDir, bucket.ID())
	size := bucketSerializedSize(len(bucket.entries))
	if err := ht.cache.write(path, size, bucket.serializeInto); err != nil {
		return err
	}
	delete(ht.dirtyBuckets, bucket.ID())
	return nil
}

func (ht *HashTable) flushDirty() error {
	for bucketID := range ht.deletedBucket {
		if err := ht.cache.remove(bucketFilePath(ht.baseDir, bucketID)); err != nil {
			return err
		}
		delete(ht.deletedBucket, bucketID)
	}

	for bucketID := range ht.dirtyBuckets {
		bucket, ok := ht.buckets[bucketID]
		if !ok {
			return fmt.Errorf("missing dirty bucket: %d", bucketID)
		}
		if err := ht.flushBucket(bucket); err != nil {
			return err
		}
	}

	if ht.metaDirty {
		if err := ht.flushMeta(); err != nil {
			return err
		}
	}

	return nil
}

func (ht *HashTable) newBucket(localDepth uint64) *Bucket {
	id := ht.meta.NextBucketID
	ht.meta.NextBucketID++
	ht.metaDirty = true

	b := NewBucket(id, localDepth)
	ht.buckets[id] = b
	ht.markBucketDirty(id)
	return b
}

func (ht *HashTable) Get(key int64) (int64, bool) {
	bucket, _, _, err := ht.bucketByKey(key)
	if err != nil || bucket == nil {
		return 0, false
	}
	return bucket.Get(key)
}

func (ht *HashTable) Delete(key int64) bool {
	bucket, bucketID, idx, err := ht.bucketByKey(key)
	if err != nil || bucket == nil {
		return false
	}

	if !bucket.Delete(key) {
		return false
	}
	ht.markBucketDirty(bucketID)

	if ht.tryMerge(bucketID, idx) {
		ht.metaDirty = true
	}

	return true
}

func (ht *HashTable) Put(key int64, value int64) {
	for {
		bucket, bucketID, _, err := ht.bucketByKey(key)
		if err != nil || bucket == nil {
			return
		}

		if _, exists := bucket.Get(key); exists {
			bucket.Put(key, value)
			ht.markBucketDirty(bucketID)
			return
		}

		if !bucket.IsFull(ht.meta.BucketLimit) {
			bucket.Put(key, value)
			ht.markBucketDirty(bucketID)
			return
		}

		ht.splitBucket(bucketID)
	}
}

func (ht *HashTable) splitBucket(oldBucketID uint64) {
	oldBucket, err := ht.bucketByID(oldBucketID)
	if err != nil || oldBucket == nil {
		return
	}

	oldLocalDepth := oldBucket.LocalDepth()
	if oldLocalDepth == ht.meta.GlobalDepth {
		ht.meta.DoubleDirectory()
		ht.metaDirty = true
	}

	newBucket := ht.newBucket(oldLocalDepth + 1)
	oldBucket.SetLocalDepth(oldLocalDepth + 1)
	ht.markBucketDirty(oldBucketID)

	ht.meta.RepointAfterSplit(oldBucketID, newBucket.ID(), oldLocalDepth)
	ht.metaDirty = true

	oldEntries := oldBucket.Entries()
	oldBucket.Clear()
	newBucket.Clear()

	for _, entry := range oldEntries {
		hash := hashKey(entry.Key)
		idx := ht.meta.Index(hash)
		targetBucketID := ht.meta.GetBucketID(idx)
		if targetBucketID == oldBucketID {
			oldBucket.entries = append(oldBucket.entries, entry)
		} else {
			newBucket.entries = append(newBucket.entries, entry)
		}
	}

	ht.markBucketDirty(oldBucketID)
	ht.markBucketDirty(newBucket.ID())
}

func (ht *HashTable) tryMerge(bucketID uint64, directoryIdx uint64) bool {
	bucket, err := ht.bucketByID(bucketID)
	if err != nil || bucket == nil {
		return false
	}
	if bucket.LocalDepth() <= startDepth {
		return false
	}

	buddyIdx := directoryIdx ^ (1 << (bucket.LocalDepth() - 1))
	buddyID := ht.meta.GetBucketID(buddyIdx)
	if buddyID == 0 || buddyID == bucketID {
		return false
	}

	buddy, err := ht.bucketByID(buddyID)
	if err != nil || buddy == nil {
		return false
	}
	if buddy.LocalDepth() != bucket.LocalDepth() {
		return false
	}
	if bucket.Count()+buddy.Count() > ht.meta.BucketLimit {
		return false
	}

	mergeIntoID := buddyID
	removeID := bucketID
	mergeInto := buddy
	removeBucket := bucket

	if buddyID > bucketID {
		mergeIntoID = bucketID
		removeID = buddyID
		mergeInto = bucket
		removeBucket = buddy
	}

	mergeInto.entries = append(mergeInto.entries, removeBucket.entries...)
	mergeInto.SetLocalDepth(mergeInto.LocalDepth() - 1)
	for i := range ht.meta.Directory {
		if ht.meta.Directory[i] == removeID {
			ht.meta.Directory[i] = mergeIntoID
		}
	}

	delete(ht.buckets, removeID)
	ht.markBucketDeleted(removeID)
	ht.markBucketDirty(mergeIntoID)

	ht.meta.ShrinkDirectory()
	ht.metaDirty = true
	return true
}

func (ht *HashTable) GlobalDepth() uint64 {
	return ht.meta.GlobalDepth
}

func (ht *HashTable) BucketCount() int {
	seen := make(map[uint64]struct{}, len(ht.meta.Directory))
	for _, bucketID := range ht.meta.Directory {
		seen[bucketID] = struct{}{}
	}
	return len(seen)
}

func (ht *HashTable) DirectorySnapshot() []uint64 {
	cp := make([]uint64, len(ht.meta.Directory))
	copy(cp, ht.meta.Directory)
	return cp
}

func (ht *HashTable) BucketLocalDepth(bucketID uint64) (uint64, bool) {
	b, err := ht.bucketByID(bucketID)
	if err != nil {
		return 0, false
	}
	return b.LocalDepth(), true
}
