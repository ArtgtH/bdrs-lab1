package tests_test

import (
	"bdrs/lab-1-perfect/perfect"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"testing"
)

func ExamplePerfectHashTable() {
	ht, err := perfect.BuildPerfectHash([]perfect.Pair{{Key: "abcd", Value: 1}, {Key: "abc", Value: 2}})
	if err != nil {
		panic(err)
	}
	fmt.Println(ht.Get("abc"))
	fmt.Println(ht.Get("abd"))
	// Output:
	// 2 true
	// 0 false
}

func TestPerfectHashEmptyTable(t *testing.T) {
	ht := perfect.NewPerfectTable()
	if ht.Len() != 0 {
		t.Fatalf("expected empty table, got len=%d", ht.Len())
	}
	if _, ok := ht.Get("missing"); ok {
		t.Fatalf("missing key must not be found in empty table")
	}
	primaryBuckets, secondarySlots := ht.Stats()
	if primaryBuckets == 0 {
		t.Fatalf("expected at least one primary bucket")
	}
	if secondarySlots != 0 {
		t.Fatalf("expected zero secondary slots for empty table, got %d", secondarySlots)
	}
}

func TestPerfectHashBuildAndLookup(t *testing.T) {
	pairs := make([]perfect.Pair, 100)
	for i := 0; i < 100; i++ {
		pairs[i] = perfect.Pair{Key: "key" + strconv.Itoa(i), Value: float64(i)}
	}
	ht, err := perfect.BuildPerfectHash(pairs)
	if err != nil {
		t.Fatalf("build perfect hash: %v", err)
	}
	if ht.Len() != 100 {
		t.Fatalf("unexpected len: got=%d want=100", ht.Len())
	}
	for i := 0; i < 100; i++ {
		v, ok := ht.Get("key" + strconv.Itoa(i))
		if !ok || v != float64(i) {
			t.Fatalf("lookup key%d: got=(%v,%v) want=(%v,true)", i, v, ok, float64(i))
		}
	}
	if _, ok := ht.Get("missing"); ok {
		t.Fatalf("missing key must not be found")
	}
}

func TestPerfectHashRejectsDuplicateKeys(t *testing.T) {
	_, err := perfect.BuildPerfectHash([]perfect.Pair{{Key: "a", Value: 1}, {Key: "a", Value: 2}})
	if err == nil {
		t.Fatalf("expected duplicate key error")
	}
}

func TestPerfectHashRandomizedAgainstSortedReference(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	keyCount := 5000
	pairs := make([]perfect.Pair, keyCount)
	expected := make(map[string]float64, keyCount)

	for i := 0; i < keyCount; i++ {
		key := fmt.Sprintf("key-%08d", i)
		value := float64(rng.Intn(1000000)) / 10
		pairs[i] = perfect.Pair{Key: key, Value: value}
		expected[key] = value
	}

	rng.Shuffle(len(pairs), func(i, j int) { pairs[i], pairs[j] = pairs[j], pairs[i] })

	ht, err := perfect.BuildPerfectHash(pairs)
	if err != nil {
		t.Fatalf("build perfect hash: %v", err)
	}

	keys := make([]string, 0, len(expected))
	for key := range expected {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		got, ok := ht.Get(key)
		if !ok || got != expected[key] {
			t.Fatalf("key %q got=(%v,%v) want=(%v,true)", key, got, ok, expected[key])
		}
	}

	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("missing-%08d", i)
		if _, ok := ht.Get(key); ok {
			t.Fatalf("unexpected hit for %q", key)
		}
	}
}
