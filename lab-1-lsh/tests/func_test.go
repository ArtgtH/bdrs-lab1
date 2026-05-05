package tests_test

import (
	lsh "bdrs/lab-1-lsh/lsh"
	"fmt"
	"math/rand"
	"testing"
)

func ExampleIndex() {
	idx, err := lsh.NewIndex(lsh.DefaultConfig())
	if err != nil {
		panic(err)
	}
	_ = idx.Add(lsh.Document{ID: "doc1", Text: "кошка ловит мышь"})
	_ = idx.Add(lsh.Document{ID: "doc2", Text: "кошка ловит мышь"})
	matches := idx.FindDuplicates("кошка ловит мышь", 0.8)
	for _, m := range matches {
		fmt.Printf("%s: %.2f\n", m.ID, m.Score)
	}
	// Output:
	// doc1: 1.00
	// doc2: 1.00
}

func TestLSHConfigValidation(t *testing.T) {
	cfg := lsh.DefaultConfig()
	cfg.NumHashes = 0
	if _, err := lsh.NewIndex(cfg); err == nil {
		t.Fatalf("expected error for NumHashes=0")
	}
	cfg = lsh.DefaultConfig()
	cfg.Bands = 0
	if _, err := lsh.NewIndex(cfg); err == nil {
		t.Fatalf("expected error for Bands=0")
	}
	cfg = lsh.DefaultConfig()
	cfg.ShingleSize = 0
	if _, err := lsh.NewIndex(cfg); err == nil {
		t.Fatalf("expected error for ShingleSize=0")
	}
	cfg = lsh.DefaultConfig()
	cfg.NumHashes = 10
	cfg.Bands = 3
	if _, err := lsh.NewIndex(cfg); err == nil {
		t.Fatalf("expected error when NumHashes is not divisible by Bands")
	}
	cfg = lsh.DefaultConfig()
	cfg.SimilarityThreshold = 1.5
	if _, err := lsh.NewIndex(cfg); err == nil {
		t.Fatalf("expected error for threshold > 1")
	}
}

func TestLSHBuildAndFindExactDuplicates(t *testing.T) {
	cfg := lsh.DefaultConfig()
	cfg.NumHashes = 32
	cfg.Bands = 8

	docs := []lsh.Document{
		{ID: "doc1", Text: "hello world"},
		{ID: "doc2", Text: "hello world"},
		{ID: "doc3", Text: "hello brave world"},
		{ID: "doc4", Text: "goodbye moon"},
	}

	idx, err := lsh.Build(docs, cfg)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	stats := idx.Stats()
	if stats.DocumentCount != len(docs) {
		t.Fatalf("unexpected document count: got=%d want=%d", stats.DocumentCount, len(docs))
	}
	if stats.BandCount != cfg.Bands {
		t.Fatalf("unexpected band count: got=%d want=%d", stats.BandCount, cfg.Bands)
	}
	if stats.NumHashes != cfg.NumHashes {
		t.Fatalf("unexpected num hashes: got=%d want=%d", stats.NumHashes, cfg.NumHashes)
	}

	matches := idx.FindDuplicates("hello world", 0.8)
	if len(matches) != 2 {
		t.Fatalf("expected 2 exact matches, got %d", len(matches))
	}
	if matches[0].ID != "doc1" || matches[0].Score != 1.0 {
		t.Fatalf("unexpected first match: %+v", matches[0])
	}
	if matches[1].ID != "doc2" || matches[1].Score != 1.0 {
		t.Fatalf("unexpected second match: %+v", matches[1])
	}
}

func TestLSHRejectsEmptyAndDuplicateID(t *testing.T) {
	idx, err := lsh.NewIndex(lsh.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}
	if err := idx.Add(lsh.Document{ID: "", Text: "text"}); err == nil {
		t.Fatalf("expected error for empty document id")
	}
	if err := idx.Add(lsh.Document{ID: "doc1", Text: "hello world"}); err != nil {
		t.Fatalf("unexpected add error: %v", err)
	}
	if err := idx.Add(lsh.Document{ID: "doc1", Text: "another text"}); err == nil {
		t.Fatalf("expected error for duplicate document id")
	}
}

func TestLSHFullScanSortsByScoreThenID(t *testing.T) {
	idx, err := lsh.NewIndex(lsh.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}
	docs := []lsh.Document{
		{ID: "a", Text: "one two three four"},
		{ID: "b", Text: "one two three four"},
		{ID: "c", Text: "one two three five"},
		{ID: "d", Text: "alpha beta gamma delta"},
	}
	for _, doc := range docs {
		if err := idx.Add(doc); err != nil {
			t.Fatalf("add failed for %s: %v", doc.ID, err)
		}
	}
	matches := idx.FullScanDuplicates("one two three four", 0.3)
	if len(matches) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(matches))
	}
	if matches[0].ID != "a" || matches[0].Score != 1.0 {
		t.Fatalf("unexpected first match: %+v", matches[0])
	}
	if matches[1].ID != "b" || matches[1].Score != 1.0 {
		t.Fatalf("unexpected second match: %+v", matches[1])
	}
	if matches[2].ID != "c" {
		t.Fatalf("expected near-duplicate as third match, got %+v", matches[2])
	}
}

func TestLSHRandomizedAgainstFullScan(t *testing.T) {
	cfg := lsh.DefaultConfig()
	cfg.NumHashes = 64
	cfg.Bands = 8
	idx, err := lsh.NewIndex(cfg)
	if err != nil {
		t.Fatalf("new index: %v", err)
	}

	rng := rand.New(rand.NewSource(123))
	vocab := []string{"alpha", "beta", "gamma", "delta", "omega", "kappa", "sigma", "tau", "lambda", "zeta"}
	docs := make([]lsh.Document, 0, 2000)
	for i := 0; i < 2000; i++ {
		n := 6 + rng.Intn(8)
		text := ""
		for j := 0; j < n; j++ {
			if j > 0 {
				text += " "
			}
			text += vocab[rng.Intn(len(vocab))]
		}
		if i%50 == 1 {
			text = docs[i-1].Text
		}
		doc := lsh.Document{ID: fmt.Sprintf("doc-%d", i), Text: text}
		docs = append(docs, doc)
		if err := idx.Add(doc); err != nil {
			t.Fatalf("add failed: %v", err)
		}
	}

	for i := 0; i < 200; i++ {
		q := docs[rng.Intn(len(docs))].Text
		got := idx.FindDuplicates(q, 0.7)
		full := idx.FullScanDuplicates(q, 0.7)
		gotSet := make(map[string]float64, len(got))
		for _, m := range got {
			gotSet[m.ID] = m.Score
		}
		for _, m := range full {
			score, ok := gotSet[m.ID]
			if !ok || score != m.Score {
				t.Fatalf("candidate search missed full-scan match: id=%s score=%f", m.ID, m.Score)
			}
		}
	}
}
