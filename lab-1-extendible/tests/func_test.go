package tests_test

import (
	"bdrs/lab-1-extendible/extendible"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math/rand"
	"path/filepath"
	"testing"
)

func ExampleHashTable() {
	ht := extendible.NewHashTable(4)
	defer ht.Close()

	ht.Put(1, 1)
	v, ok := ht.Get(1)
	fmt.Println(v, ok)
	fmt.Println(ht.Delete(1))
	fmt.Println(ht.Get(1))

	// Output:
	// 1 true
	// true
	// 0 false
}

func mustNewTable(t *testing.T, path string, bucketLimit uint64) *extendible.HashTable {
	t.Helper()
	ht, err := extendible.NewHashTableOnDisk(path, bucketLimit)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	return ht
}

func mustOpenTable(t *testing.T, path string) *extendible.HashTable {
	t.Helper()
	ht, err := extendible.OpenHashTable(path)
	if err != nil {
		t.Fatalf("open table: %v", err)
	}
	return ht
}

func mustCloseTable(t *testing.T, ht *extendible.HashTable) {
	t.Helper()
	if err := ht.Close(); err != nil {
		t.Fatalf("close table: %v", err)
	}
}

func reopenTable(t *testing.T, ht *extendible.HashTable, path string) *extendible.HashTable {
	t.Helper()
	mustCloseTable(t, ht)
	return mustOpenTable(t, path)
}

func hashKeyForTest(key int64) uint64 {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(key))
	h := fnv.New64a()
	_, _ = h.Write(buf[:])
	return h.Sum64()
}

func verifyHashTableState(t *testing.T, ht *extendible.HashTable, ref map[int64]int64) {
	t.Helper()

	globalDepth := ht.GlobalDepth()
	directory := ht.DirectorySnapshot()

	if uint64(len(directory)) != (uint64(1) << globalDepth) {
		t.Fatalf("directory size mismatch: got=%d want=%d globalDepth=%d", len(directory), uint64(1)<<globalDepth, globalDepth)
	}

	refCounts := make(map[uint64]uint64)
	for i, bucketID := range directory {
		if bucketID == 0 {
			t.Fatalf("directory[%d] contains zero bucket id", i)
		}
		refCounts[bucketID]++
	}

	if ht.BucketCount() != len(refCounts) {
		t.Fatalf("bucket count mismatch: got=%d want=%d", ht.BucketCount(), len(refCounts))
	}

	for bucketID, count := range refCounts {
		localDepth, ok := ht.BucketLocalDepth(bucketID)
		if !ok {
			t.Fatalf("cannot read local depth for bucket=%d", bucketID)
		}
		if localDepth > globalDepth {
			t.Fatalf("bucket=%d localDepth=%d > globalDepth=%d", bucketID, localDepth, globalDepth)
		}

		wantRefs := uint64(1) << (globalDepth - localDepth)
		if count != wantRefs {
			t.Fatalf("bucket=%d reference count mismatch: got=%d want=%d localDepth=%d globalDepth=%d", bucketID, count, wantRefs, localDepth, globalDepth)
		}
	}

	for key, wantValue := range ref {
		gotValue, ok := ht.Get(key)
		if !ok {
			t.Fatalf("missing key=%d", key)
		}
		if gotValue != wantValue {
			t.Fatalf("value mismatch for key=%d: got=%d want=%d", key, gotValue, wantValue)
		}

		idx := hashKeyForTest(key) & ((uint64(1) << globalDepth) - 1)
		if directory[idx] == 0 {
			t.Fatalf("key=%d mapped to zero bucket id", key)
		}
	}
}

func TestHashTableRandomizedAgainstMap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "table")
	ht := mustNewTable(t, path, 8)

	seed := int64(123)
	rng := rand.New(rand.NewSource(seed))
	ref := make(map[int64]int64)

	const ops = 20000
	const keySpace = 1000
	const valueSpace = 1_000_000

	for i := 0; i < ops; i++ {
		op := rng.Intn(3)
		key := int64(rng.Intn(keySpace))
		value := rng.Int63n(valueSpace)

		switch op {
		case 0:
			ht.Put(key, value)
			ref[key] = value
		case 1:
			gotValue, gotOK := ht.Get(key)
			wantValue, wantOK := ref[key]
			if gotOK != wantOK {
				t.Fatalf("step=%d key=%d ok mismatch: got=%v want=%v", i, key, gotOK, wantOK)
			}
			if gotOK && gotValue != wantValue {
				t.Fatalf("step=%d key=%d value mismatch: got=%d want=%d", i, key, gotValue, wantValue)
			}
		case 2:
			gotDeleted := ht.Delete(key)
			_, existed := ref[key]
			if existed {
				delete(ref, key)
			}
			if gotDeleted != existed {
				t.Fatalf("step=%d key=%d delete mismatch: got=%v want=%v", i, key, gotDeleted, existed)
			}
		}

		if (i+1)%500 == 0 {
			verifyHashTableState(t, ht, ref)
			ht = reopenTable(t, ht, path)
			verifyHashTableState(t, ht, ref)
		}
	}

	verifyHashTableState(t, ht, ref)
	ht = reopenTable(t, ht, path)
	verifyHashTableState(t, ht, ref)
	mustCloseTable(t, ht)
}

func TestHashTablePersistenceReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "table")
	ht := mustNewTable(t, path, 4)
	ref := make(map[int64]int64)

	for i := int64(0); i < 256; i++ {
		ht.Put(i, i*10)
		ref[i] = i * 10
	}
	for i := int64(0); i < 256; i += 2 {
		ht.Put(i, i*100)
		ref[i] = i * 100
	}
	for i := int64(0); i < 256; i += 3 {
		gotDeleted := ht.Delete(i)
		_, existed := ref[i]
		if existed {
			delete(ref, i)
		}
		if gotDeleted != existed {
			t.Fatalf("delete key=%d mismatch: got=%v want=%v", i, gotDeleted, existed)
		}
	}

	verifyHashTableState(t, ht, ref)
	ht = reopenTable(t, ht, path)
	verifyHashTableState(t, ht, ref)
	mustCloseTable(t, ht)
}

func TestHashTableShrinkActuallyHappens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "table")
	ht := mustNewTable(t, path, 2)
	ref := make(map[int64]int64)

	for i := int64(0); i < 128; i++ {
		ht.Put(i, i)
		ref[i] = i
	}

	grownDepth := ht.GlobalDepth()
	if grownDepth <= 1 {
		t.Fatalf("expected global depth to grow, got=%d", grownDepth)
	}

	verifyHashTableState(t, ht, ref)

	for i := int64(0); i < 128; i++ {
		if !ht.Delete(i) {
			t.Fatalf("delete failed for key=%d", i)
		}
		delete(ref, i)
	}

	if ht.GlobalDepth() != 1 {
		t.Fatalf("expected global depth to shrink back to 1, got=%d", ht.GlobalDepth())
	}
	if ht.BucketCount() != 2 {
		t.Fatalf("expected bucket count to shrink back to 2, got=%d", ht.BucketCount())
	}

	verifyHashTableState(t, ht, ref)
	ht = reopenTable(t, ht, path)
	verifyHashTableState(t, ht, ref)
	mustCloseTable(t, ht)
}

func TestHashTableRejectsZeroBucketLimit(t *testing.T) {
	_, err := extendible.NewHashTableOnDisk(filepath.Join(t.TempDir(), "table"), 0)
	if err == nil {
		t.Fatalf("expected error for zero bucket limit")
	}
}

func TestHashTableOpenMissingDirectory(t *testing.T) {
	_, err := extendible.OpenHashTable(filepath.Join(t.TempDir(), "missing-table"))
	if err == nil {
		t.Fatalf("expected error when opening missing table")
	}
}
