package tests_test

import (
	lsh "bdrs/lab-1-lsh/lsh"
	"fmt"
	"testing"
)

var (
	benchmarkSizes = []int{10000, 50000, 100000, 250000}
	lshSinkCount   int
)

func benchmarkConfig() lsh.Config {
	cfg := lsh.DefaultConfig()
	cfg.NumHashes = 64
	cfg.Bands = 8
	cfg.ShingleSize = 2
	cfg.SimilarityThreshold = 0.5
	return cfg
}

func makeCorpus(size int) []lsh.Document {
	docs := make([]lsh.Document, 0, size)
	for i := 0; i < size; i++ {
		group := i / 20
		text := fmt.Sprintf("distributed systems lab document %d hashing indexing duplicates token token token", i)
		if i%20 == 0 {
			text = fmt.Sprintf("near duplicate text for group %d alpha beta gamma", group)
		}
		if i%20 == 1 {
			text = fmt.Sprintf("near duplicate text for group %d alpha beta gamma", group)
		}
		if i%20 == 2 {
			text = fmt.Sprintf("near duplicate text for group %d alpha beta delta", group)
		}
		docs = append(docs, lsh.Document{ID: fmt.Sprintf("doc-%d", i), Text: text})
	}
	return docs
}

func makeInsertedDocs(size int, offset int) []lsh.Document {
	docs := make([]lsh.Document, 0, size)
	for i := 0; i < size; i++ {
		group := (offset + i) / 10
		text := fmt.Sprintf("inserted document %d alpha beta gamma %d", offset+i, group)
		if i%10 == 0 {
			text = fmt.Sprintf("near duplicate inserted group %d alpha beta gamma", group)
		}
		if i%10 == 1 {
			text = fmt.Sprintf("near duplicate inserted group %d alpha beta delta", group)
		}
		docs = append(docs, lsh.Document{ID: fmt.Sprintf("new-%d", offset+i), Text: text})
	}
	return docs
}

func mustBuildIndex(b *testing.B, docs []lsh.Document, cfg lsh.Config) *lsh.Index {
	idx, err := lsh.Build(docs, cfg)
	if err != nil {
		b.Fatalf("build failed: %v", err)
	}
	return idx
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
	b.ReportMetric(float64(elapsed.Nanoseconds())/processed, "ns/doc")
	b.ReportMetric(processed/elapsed.Seconds(), "docs/s")
}

func BenchmarkLSHBuild(b *testing.B) {
	cfg := benchmarkConfig()
	for _, size := range benchmarkSizes {
		docs := makeCorpus(size)
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			runBatchBenchmark(b, size, func(iter int) {
				idx := mustBuildIndex(b, docs, cfg)
				if idx.Stats().DocumentCount != size {
					b.Fatalf("unexpected document count")
				}
			})
		})
	}
}

func BenchmarkLSHAdd(b *testing.B) {
	cfg := benchmarkConfig()
	for _, size := range benchmarkSizes {
		baseDocs := makeCorpus(size)
		addCount := size / 10
		if addCount == 0 {
			addCount = 1
		}
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			runBatchBenchmark(b, addCount, func(iter int) {
				newDocs := makeInsertedDocs(addCount, iter*addCount)
				b.StopTimer()
				idx := mustBuildIndex(b, baseDocs, cfg)
				b.StartTimer()
				for _, doc := range newDocs {
					if err := idx.Add(doc); err != nil {
						b.Fatalf("add failed for %s: %v", doc.ID, err)
					}
				}
			})
		})
	}
}

func BenchmarkLSHFindDuplicates(b *testing.B) {
	cfg := benchmarkConfig()
	query := "near duplicate text for group 10 alpha beta gamma"
	for _, size := range benchmarkSizes {
		idx := mustBuildIndex(b, makeCorpus(size), cfg)
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			total := 0
			for i := 0; i < b.N; i++ {
				matches := idx.FindDuplicates(query, 0.5)
				total += len(matches)
			}
			lshSinkCount = total
			b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "ops/s")
		})
	}
}

func BenchmarkLSHFullScanDuplicates(b *testing.B) {
	cfg := benchmarkConfig()
	query := "near duplicate text for group 10 alpha beta gamma"
	for _, size := range benchmarkSizes {
		idx := mustBuildIndex(b, makeCorpus(size), cfg)
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			total := 0
			for i := 0; i < b.N; i++ {
				matches := idx.FullScanDuplicates(query, 0.5)
				total += len(matches)
			}
			lshSinkCount = total
			b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "ops/s")
		})
	}
}
