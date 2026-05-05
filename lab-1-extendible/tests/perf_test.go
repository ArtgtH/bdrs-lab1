package tests_test

import (
	ex "bdrs/lab-1-extendible/extendible"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

var (
	benchmarkSizes        = []int{10000, 50000, 100000, 250000}
	benchmarkBucketLimits = []uint64{32, 64, 128}
	extendibleSinkValue   int64
)

func makeRangeKeys(n int, start int64) []int64 {
	keys := make([]int64, n)
	for i := range keys {
		keys[i] = start + int64(i)
	}
	return keys
}

func mustNewBenchmarkTable(b *testing.B, path string, bucketLimit uint64) *ex.HashTable {
	ht, err := ex.NewHashTableOnDisk(path, bucketLimit)
	if err != nil {
		b.Fatalf("create table: %v", err)
	}
	return ht
}

func mustOpenBenchmarkTable(b *testing.B, path string) *ex.HashTable {
	ht, err := ex.OpenHashTable(path)
	if err != nil {
		b.Fatalf("open table: %v", err)
	}
	return ht
}

func prepareBenchmarkTable(b *testing.B, path string, bucketLimit uint64, keys []int64, reopen bool) *ex.HashTable {
	ht := mustNewBenchmarkTable(b, path, bucketLimit)
	for i, key := range keys {
		ht.Put(key, int64(i))
	}
	if err := ht.Sync(); err != nil {
		b.Fatalf("sync prepared table: %v", err)
	}
	if reopen {
		if err := ht.Close(); err != nil {
			b.Fatalf("close prepared table: %v", err)
		}
		ht = mustOpenBenchmarkTable(b, path)
	}
	return ht
}

func runBatchBenchmark(b *testing.B, batchSize int, fn func(iter int)) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fn(i)
	}
	elapsed := b.Elapsed()
	processed := float64(batchSize * b.N)
	if processed <= 0 || elapsed <= 0 {
		return
	}
	b.ReportMetric(float64(elapsed.Nanoseconds())/processed, "ns/item")
	b.ReportMetric(processed/elapsed.Seconds(), "ops/s")
}

func BenchmarkExtendibleBuild(b *testing.B) {
	for _, size := range benchmarkSizes {
		keys := makeRangeKeys(size, 0)
		for _, limit := range benchmarkBucketLimits {
			b.Run(fmt.Sprintf("size=%d/limit=%d", size, limit), func(b *testing.B) {
				baseDir := b.TempDir()
				runBatchBenchmark(b, size, func(iter int) {
					path := filepath.Join(baseDir, fmt.Sprintf("build-%d-%d-%d", size, limit, iter))
					b.StopTimer()
					ht := mustNewBenchmarkTable(b, path, limit)
					b.StartTimer()
					for i, key := range keys {
						ht.Put(key, int64(i))
					}
					b.StopTimer()
					if err := ht.Close(); err != nil {
						b.Fatalf("close table: %v", err)
					}
					if err := os.RemoveAll(path); err != nil {
						b.Fatalf("remove table dir: %v", err)
					}
				})
			})
		}
	}
}

func BenchmarkExtendibleGet(b *testing.B) {
	for _, size := range benchmarkSizes {
		keys := makeRangeKeys(size, 0)
		for _, limit := range benchmarkBucketLimits {
			b.Run(fmt.Sprintf("size=%d/limit=%d", size, limit), func(b *testing.B) {
				baseDir := b.TempDir()
				path := filepath.Join(baseDir, fmt.Sprintf("get-%d-%d", size, limit))
				b.StopTimer()
				ht := prepareBenchmarkTable(b, path, limit, keys, true)
				if _, ok := ht.Get(keys[0]); !ok {
					b.Fatalf("warmup get key=%d returned false", keys[0])
				}
				b.StartTimer()
				var sum int64
				for i := 0; i < b.N; i++ {
					key := keys[i%len(keys)]
					value, ok := ht.Get(key)
					if !ok {
						b.Fatalf("get key=%d returned false", key)
					}
					sum += value
				}
				extendibleSinkValue = sum
				b.StopTimer()
				if err := ht.Close(); err != nil {
					b.Fatalf("close table: %v", err)
				}
				b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "ops/s")
			})
		}
	}
}

func BenchmarkExtendibleInsert(b *testing.B) {
	for _, size := range benchmarkSizes {
		baseKeys := makeRangeKeys(size, 0)
		insertKeys := makeRangeKeys(size, int64(size))
		for _, limit := range benchmarkBucketLimits {
			b.Run(fmt.Sprintf("size=%d/limit=%d", size, limit), func(b *testing.B) {
				baseDir := b.TempDir()
				runBatchBenchmark(b, size, func(iter int) {
					path := filepath.Join(baseDir, fmt.Sprintf("insert-%d-%d-%d", size, limit, iter))
					b.StopTimer()
					ht := prepareBenchmarkTable(b, path, limit, baseKeys, false)
					b.StartTimer()
					for i, key := range insertKeys {
						ht.Put(key, int64(i+size))
					}
					b.StopTimer()
					if err := ht.Close(); err != nil {
						b.Fatalf("close table: %v", err)
					}
					if err := os.RemoveAll(path); err != nil {
						b.Fatalf("remove table dir: %v", err)
					}
				})
			})
		}
	}
}

func BenchmarkExtendibleUpdate(b *testing.B) {
	for _, size := range benchmarkSizes {
		keys := makeRangeKeys(size, 0)
		for _, limit := range benchmarkBucketLimits {
			b.Run(fmt.Sprintf("size=%d/limit=%d", size, limit), func(b *testing.B) {
				baseDir := b.TempDir()
				runBatchBenchmark(b, size, func(iter int) {
					path := filepath.Join(baseDir, fmt.Sprintf("update-%d-%d-%d", size, limit, iter))
					b.StopTimer()
					ht := prepareBenchmarkTable(b, path, limit, keys, false)
					b.StartTimer()
					for i, key := range keys {
						ht.Put(key, int64(i+size))
					}
					b.StopTimer()
					if err := ht.Close(); err != nil {
						b.Fatalf("close table: %v", err)
					}
					if err := os.RemoveAll(path); err != nil {
						b.Fatalf("remove table dir: %v", err)
					}
				})
			})
		}
	}
}

func BenchmarkExtendibleDelete(b *testing.B) {
	for _, size := range benchmarkSizes {
		keys := makeRangeKeys(size, 0)
		for _, limit := range benchmarkBucketLimits {
			b.Run(fmt.Sprintf("size=%d/limit=%d", size, limit), func(b *testing.B) {
				baseDir := b.TempDir()
				runBatchBenchmark(b, size, func(iter int) {
					path := filepath.Join(baseDir, fmt.Sprintf("delete-%d-%d-%d", size, limit, iter))
					b.StopTimer()
					ht := prepareBenchmarkTable(b, path, limit, keys, false)
					b.StartTimer()
					for _, key := range keys {
						if !ht.Delete(key) {
							b.Fatalf("delete key=%d returned false", key)
						}
					}
					b.StopTimer()
					if err := ht.Close(); err != nil {
						b.Fatalf("close table: %v", err)
					}
					if err := os.RemoveAll(path); err != nil {
						b.Fatalf("remove table dir: %v", err)
					}
				})
			})
		}
	}
}
