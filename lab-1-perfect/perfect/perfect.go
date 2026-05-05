package perfect

import (
	"errors"
	"fmt"
	"sort"
)

type Pair struct {
	Key   string
	Value float64
}

type slotEntry struct {
	key   string
	value float64
	used  bool
}

type secondaryBucket struct {
	seed   uint64
	offset int
	size   int
	count  int
}

type PerfectHashTable struct {
	primarySeed uint64
	buckets     []secondaryBucket
	slots       []slotEntry
	size        int
}

func NewPerfectTable() *PerfectHashTable {
	return &PerfectHashTable{
		buckets: make([]secondaryBucket, 1),
	}
}

func BuildPerfectHash(pairs []Pair) (*PerfectHashTable, error) {
	normalized, err := normalizePairs(pairs)
	if err != nil {
		return nil, err
	}
	if len(normalized) == 0 {
		return NewPerfectTable(), nil
	}

	bestSeed, counts := choosePrimarySeed(normalized)
	starts := prefixSums(counts)
	order := distributePairs(normalized, bestSeed, counts, starts)

	buckets := make([]secondaryBucket, len(normalized))
	totalSlots := 0
	for i, count := range counts {
		if count == 0 {
			continue
		}
		if count == 1 {
			totalSlots++
			continue
		}
		totalSlots += count * count
		_ = i
	}

	slots := make([]slotEntry, totalSlots)
	cursor := 0
	for i, count := range counts {
		bucketStart := starts[i]
		bucketEnd := bucketStart + count
		built, nextCursor, err := buildSecondary(normalized, order[bucketStart:bucketEnd], cursor, slots)
		if err != nil {
			return nil, fmt.Errorf("build bucket %d: %w", i, err)
		}
		buckets[i] = built
		cursor = nextCursor
	}

	return &PerfectHashTable{
		primarySeed: bestSeed,
		buckets:     buckets,
		slots:       slots,
		size:        len(normalized),
	}, nil
}

func normalizePairs(pairs []Pair) ([]Pair, error) {
	out := make([]Pair, len(pairs))
	copy(out, pairs)
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	for i := 1; i < len(out); i++ {
		if out[i-1].Key == out[i].Key {
			return nil, fmt.Errorf("duplicate key %q", out[i].Key)
		}
	}
	return out, nil
}

func choosePrimarySeed(pairs []Pair) (uint64, []int) {
	n := len(pairs)
	bestSeed := uint64(1)
	bestCounts := make([]int, n)
	bestScore := uint64(^uint64(0))
	limit := uint64(4 * n)
	if limit == 0 {
		limit = 1
	}

	counts := make([]int, n)
	for attempt := uint64(0); attempt < 256; attempt++ {
		for i := range counts {
			counts[i] = 0
		}
		seed := mix64(attempt + 1)
		for i := range pairs {
			idx := int(hashWithSeed(pairs[i].Key, seed) % uint64(n))
			counts[idx]++
		}
		var score uint64
		for _, c := range counts {
			score += uint64(c * c)
		}
		if score < bestScore {
			bestScore = score
			bestSeed = seed
			copy(bestCounts, counts)
		}
		if score <= limit {
			break
		}
	}
	return bestSeed, bestCounts
}

func prefixSums(counts []int) []int {
	starts := make([]int, len(counts)+1)
	for i, c := range counts {
		starts[i+1] = starts[i] + c
	}
	return starts
}

func distributePairs(pairs []Pair, seed uint64, counts []int, starts []int) []int {
	order := make([]int, len(pairs))
	cursor := make([]int, len(counts))
	copy(cursor, starts[:len(counts)])
	for i := range pairs {
		idx := int(hashWithSeed(pairs[i].Key, seed) % uint64(len(counts)))
		pos := cursor[idx]
		order[pos] = i
		cursor[idx]++
	}
	return order
}

func buildSecondary(pairs []Pair, group []int, offset int, slots []slotEntry) (secondaryBucket, int, error) {
	switch len(group) {
	case 0:
		return secondaryBucket{}, offset, nil
	case 1:
		slots[offset] = slotEntry{key: pairs[group[0]].Key, value: pairs[group[0]].Value, used: true}
		return secondaryBucket{seed: 0, offset: offset, size: 1, count: 1}, offset + 1, nil
	}

	size := len(group) * len(group)
	if size <= 0 {
		return secondaryBucket{}, offset, errors.New("invalid secondary size")
	}

	stamps := make([]uint32, size)
	touched := make([]int, len(group))
	epoch := uint32(1)

	for attempt := uint64(0); attempt < 10000; attempt++ {
		seed := mix64(uint64(len(group)) + attempt + 1)
		collision := false

		for i, pairIdx := range group {
			pos := int(hashWithSeed(pairs[pairIdx].Key, seed) % uint64(size))
			if stamps[pos] == epoch {
				collision = true
				break
			}
			stamps[pos] = epoch
			touched[i] = pos
		}

		if !collision {
			end := offset + size
			bucketSlots := slots[offset:end]
			for i := range bucketSlots {
				bucketSlots[i] = slotEntry{}
			}
			for i, pairIdx := range group {
				bucketSlots[touched[i]] = slotEntry{
					key:   pairs[pairIdx].Key,
					value: pairs[pairIdx].Value,
					used:  true,
				}
			}
			return secondaryBucket{seed: seed, offset: offset, size: size, count: len(group)}, end, nil
		}

		epoch++
		if epoch != 0 {
			continue
		}
		for i := range stamps {
			stamps[i] = 0
		}
		epoch = 1
	}

	return secondaryBucket{}, offset, fmt.Errorf("failed to find collision-free secondary seed for %d keys", len(group))
}

func (ht *PerfectHashTable) Get(key string) (float64, bool) {
	if len(ht.buckets) == 0 {
		return 0, false
	}

	primaryIdx := int(hashWithSeed(key, ht.primarySeed) % uint64(len(ht.buckets)))
	bucket := ht.buckets[primaryIdx]
	if bucket.size == 0 {
		return 0, false
	}

	if bucket.size == 1 {
		entry := ht.slots[bucket.offset]
		if entry.used && entry.key == key {
			return entry.value, true
		}
		return 0, false
	}

	secondaryIdx := int(hashWithSeed(key, bucket.seed) % uint64(bucket.size))
	entry := ht.slots[bucket.offset+secondaryIdx]
	if !entry.used || entry.key != key {
		return 0, false
	}
	return entry.value, true
}

func (ht *PerfectHashTable) Len() int {
	return ht.size
}

func (ht *PerfectHashTable) Stats() (primaryBuckets int, secondarySlots int) {
	primaryBuckets = len(ht.buckets)
	secondarySlots = len(ht.slots)
	return primaryBuckets, secondarySlots
}
