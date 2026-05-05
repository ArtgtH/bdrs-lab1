package tests_test

import (
	"bdrs/lab-1-perfect/perfect"
	"fmt"
	"strconv"
	"testing"
)

var (
	benchmarkSizes   = []int{10000, 50000, 100000, 250000}
	perfectSinkValue float64
)

func makePairs(n int) []perfect.Pair {
	pairs := make([]perfect.Pair, n)
	for i := 0; i < n; i++ {
		pairs[i] = perfect.Pair{Key: "key" + strconv.Itoa(i), Value: float64(i)}
	}
	return pairs
}

func mustBuildPerfectHash(b *testing.B, pairs []perfect.Pair) *perfect.PerfectHashTable {
	ht, err := perfect.BuildPerfectHash(pairs)
	if err != nil {
		b.Fatalf("build perfect hash: %v", err)
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
	b.ReportMetric(processed/elapsed.Seconds(), "items/s")
}

func BenchmarkPerfectBuild(b *testing.B) {
	for _, size := range benchmarkSizes {
		pairs := makePairs(size)
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			runBatchBenchmark(b, size, func(iter int) {
				ht := mustBuildPerfectHash(b, pairs)
				if ht.Len() != size {
					b.Fatalf("unexpected size: got=%d want=%d", ht.Len(), size)
				}
			})
		})
	}
}

func BenchmarkPerfectGet(b *testing.B) {
	for _, size := range benchmarkSizes {
		pairs := makePairs(size)
		ht := mustBuildPerfectHash(b, pairs)
		keys := make([]string, len(pairs))
		for i := range pairs {
			keys[i] = pairs[i].Key
		}
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			var sum float64
			for i := 0; i < b.N; i++ {
				value, ok := ht.Get(keys[i%len(keys)])
				if !ok {
					b.Fatalf("existing key lookup failed")
				}
				sum += value
			}
			perfectSinkValue = sum
			b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "ops/s")
		})
	}
}
