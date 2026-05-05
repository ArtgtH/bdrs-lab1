package lsh

import (
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"unicode"
	"unicode/utf8"
)

const (
	DefaultNumHashes           = 64
	DefaultBands               = 8
	DefaultShingleSize         = 2
	DefaultSimilarityThreshold = 0.8
)

var emptyDocumentHash = hashBytes64([]byte("<empty>"))

type Config struct {
	NumHashes           int
	Bands               int
	ShingleSize         int
	SimilarityThreshold float64
}

type Document struct {
	ID   string
	Text string
}

type Match struct {
	ID    string
	Score float64
}

type storedDocument struct {
	ID       string
	Shingles []uint64
}

type Index struct {
	cfg           Config
	seeds         []uint64
	rowsPerBand   int
	docIndex      map[string]int
	docs          []storedDocument
	buckets       []map[uint64][]int
	candidateMark []uint32
	epoch         uint32
}

func DefaultConfig() Config {
	return Config{
		NumHashes:           DefaultNumHashes,
		Bands:               DefaultBands,
		ShingleSize:         DefaultShingleSize,
		SimilarityThreshold: DefaultSimilarityThreshold,
	}
}

func NewIndex(cfg Config) (*Index, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	idx := &Index{
		cfg:         cfg,
		seeds:       generateSeeds(cfg.NumHashes),
		rowsPerBand: cfg.NumHashes / cfg.Bands,
		docIndex:    make(map[string]int),
		buckets:     make([]map[uint64][]int, cfg.Bands),
	}
	for i := range idx.buckets {
		idx.buckets[i] = make(map[uint64][]int)
	}
	return idx, nil
}

func Build(docs []Document, cfg Config) (*Index, error) {
	idx, err := NewIndex(cfg)
	if err != nil {
		return nil, err
	}
	idx.reserve(len(docs))
	for _, doc := range docs {
		if err := idx.Add(doc); err != nil {
			return nil, err
		}
	}
	return idx, nil
}

func (cfg Config) validate() error {
	if cfg.NumHashes <= 0 {
		return errors.New("num hashes must be positive")
	}
	if cfg.Bands <= 0 {
		return errors.New("bands must be positive")
	}
	if cfg.ShingleSize <= 0 {
		return errors.New("shingle size must be positive")
	}
	if cfg.NumHashes%cfg.Bands != 0 {
		return fmt.Errorf("num hashes (%d) must be divisible by bands (%d)", cfg.NumHashes, cfg.Bands)
	}
	if cfg.SimilarityThreshold < 0 || cfg.SimilarityThreshold > 1 {
		return errors.New("similarity threshold must be in [0,1]")
	}
	return nil
}

func generateSeeds(count int) []uint64 {
	state := uint64(0x9e3779b97f4a7c15)
	seeds := make([]uint64, count)
	for i := 0; i < count; i++ {
		state += 0x9e3779b97f4a7c15
		seeds[i] = mix64(state)
	}
	return seeds
}

func (idx *Index) reserve(n int) {
	if n <= 0 {
		return
	}
	if cap(idx.docs) < n {
		idx.docs = make([]storedDocument, 0, n)
	}
	if len(idx.candidateMark) < n {
		idx.candidateMark = make([]uint32, n)
	}
	if idx.docIndex == nil {
		idx.docIndex = make(map[string]int, n)
	}
}

func hashBytes64(value []byte) uint64 {
	h := fnv.New64a()
	_, _ = h.Write(value)
	return h.Sum64()
}

func normalizedTokenHashes(text string) []uint64 {
	tokens := make([]uint64, 0, 16)
	buf := make([]byte, 0, 32)
	flush := func() {
		if len(buf) == 0 {
			return
		}
		tokens = append(tokens, hashBytes64(buf))
		buf = buf[:0]
	}

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			buf = utf8.AppendRune(buf, unicode.ToLower(r))
			continue
		}
		flush()
	}
	flush()
	return tokens
}

func combineTokenHashes(tokens []uint64, start, end int) uint64 {
	h := uint64(1469598103934665603)
	for i := start; i < end; i++ {
		h ^= mix64(tokens[i] + 0x9e3779b97f4a7c15)
		h *= 1099511628211
	}
	return h
}

func shingleHashes(text string, shingleSize int) []uint64 {
	tokens := normalizedTokenHashes(text)
	if len(tokens) == 0 {
		return []uint64{emptyDocumentHash}
	}
	if len(tokens) < shingleSize {
		return []uint64{combineTokenHashes(tokens, 0, len(tokens))}
	}

	count := len(tokens) - shingleSize + 1
	shingles := make([]uint64, count)
	for i := 0; i < count; i++ {
		shingles[i] = combineTokenHashes(tokens, i, i+shingleSize)
	}
	sort.Slice(shingles, func(i, j int) bool { return shingles[i] < shingles[j] })
	unique := 1
	for i := 1; i < len(shingles); i++ {
		if shingles[i] == shingles[unique-1] {
			continue
		}
		shingles[unique] = shingles[i]
		unique++
	}
	return shingles[:unique]
}

func mix64(value uint64) uint64 {
	value ^= value >> 30
	value *= 0xbf58476d1ce4e5b9
	value ^= value >> 27
	value *= 0x94d049bb133111eb
	value ^= value >> 31
	return value
}

func (idx *Index) signatureFor(shingles []uint64) []uint64 {
	sig := make([]uint64, idx.cfg.NumHashes)
	for i := range sig {
		sig[i] = math.MaxUint64
		seed := idx.seeds[i]
		for _, shingle := range shingles {
			value := mix64(shingle ^ seed)
			if value < sig[i] {
				sig[i] = value
			}
		}
	}
	return sig
}

func bandKey(values []uint64) uint64 {
	h := uint64(1469598103934665603)
	for _, v := range values {
		h ^= mix64(v)
		h *= 1099511628211
	}
	return h
}

func (idx *Index) ensureCandidateMarks() {
	if len(idx.candidateMark) < len(idx.docs) {
		marks := make([]uint32, len(idx.docs))
		copy(marks, idx.candidateMark)
		idx.candidateMark = marks
	}
	idx.epoch++
	if idx.epoch != 0 {
		return
	}
	for i := range idx.candidateMark {
		idx.candidateMark[i] = 0
	}
	idx.epoch = 1
}

func (idx *Index) Add(doc Document) error {
	if doc.ID == "" {
		return errors.New("document id must not be empty")
	}
	if _, exists := idx.docIndex[doc.ID]; exists {
		return fmt.Errorf("document %q already exists", doc.ID)
	}

	shingles := shingleHashes(doc.Text, idx.cfg.ShingleSize)
	signature := idx.signatureFor(shingles)
	docIdx := len(idx.docs)
	idx.docs = append(idx.docs, storedDocument{ID: doc.ID, Shingles: shingles})
	idx.docIndex[doc.ID] = docIdx
	if len(idx.candidateMark) < len(idx.docs) {
		idx.candidateMark = append(idx.candidateMark, 0)
	}

	for band := 0; band < idx.cfg.Bands; band++ {
		start := band * idx.rowsPerBand
		end := start + idx.rowsPerBand
		key := bandKey(signature[start:end])
		idx.buckets[band][key] = append(idx.buckets[band][key], docIdx)
	}
	return nil
}

func (idx *Index) candidateIndicesFromSignature(signature []uint64) []int {
	idx.ensureCandidateMarks()
	candidates := make([]int, 0, 16)
	for band := 0; band < idx.cfg.Bands; band++ {
		start := band * idx.rowsPerBand
		end := start + idx.rowsPerBand
		key := bandKey(signature[start:end])
		for _, docIdx := range idx.buckets[band][key] {
			if idx.candidateMark[docIdx] == idx.epoch {
				continue
			}
			idx.candidateMark[docIdx] = idx.epoch
			candidates = append(candidates, docIdx)
		}
	}
	return candidates
}

func (idx *Index) CandidateIDs(text string) []string {
	shingles := shingleHashes(text, idx.cfg.ShingleSize)
	signature := idx.signatureFor(shingles)
	candidates := idx.candidateIndicesFromSignature(signature)
	ids := make([]string, len(candidates))
	for i, docIdx := range candidates {
		ids[i] = idx.docs[docIdx].ID
	}
	sort.Strings(ids)
	return ids
}

func (idx *Index) FindDuplicates(text string, threshold float64) []Match {
	threshold = idx.resolveThreshold(threshold)
	queryShingles := shingleHashes(text, idx.cfg.ShingleSize)
	signature := idx.signatureFor(queryShingles)
	candidates := idx.candidateIndicesFromSignature(signature)
	matches := make([]Match, 0, len(candidates))
	for _, docIdx := range candidates {
		doc := idx.docs[docIdx]
		score := jaccardSorted(queryShingles, doc.Shingles)
		if score >= threshold {
			matches = append(matches, Match{ID: doc.ID, Score: score})
		}
	}
	return sortMatches(matches)
}

func jaccardSorted(left, right []uint64) float64 {
	if len(left) == 0 && len(right) == 0 {
		return 1.0
	}
	i, j := 0, 0
	intersection := 0
	for i < len(left) && j < len(right) {
		switch {
		case left[i] == right[j]:
			intersection++
			i++
			j++
		case left[i] < right[j]:
			i++
		default:
			j++
		}
	}
	union := len(left) + len(right) - intersection
	if union == 0 {
		return 1.0
	}
	return float64(intersection) / float64(union)
}

func (idx *Index) resolveThreshold(threshold float64) float64 {
	if threshold == 0 {
		return idx.cfg.SimilarityThreshold
	}
	if threshold < 0 {
		return 0
	}
	if threshold > 1 {
		return 1
	}
	return threshold
}

func sortMatches(matches []Match) []Match {
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score == matches[j].Score {
			return matches[i].ID < matches[j].ID
		}
		return matches[i].Score > matches[j].Score
	})
	return matches
}

func (idx *Index) FullScanDuplicates(text string, threshold float64) []Match {
	threshold = idx.resolveThreshold(threshold)
	queryShingles := shingleHashes(text, idx.cfg.ShingleSize)
	matches := make([]Match, 0, len(idx.docs))
	for i := range idx.docs {
		doc := idx.docs[i]
		score := jaccardSorted(queryShingles, doc.Shingles)
		if score >= threshold {
			matches = append(matches, Match{ID: doc.ID, Score: score})
		}
	}
	return sortMatches(matches)
}

type Stats struct {
	DocumentCount int
	BucketCount   int
	BandCount     int
	NumHashes     int
	ShingleSize   int
}

func (idx *Index) Stats() Stats {
	bucketCount := 0
	for _, band := range idx.buckets {
		bucketCount += len(band)
	}
	return Stats{
		DocumentCount: len(idx.docs),
		BucketCount:   bucketCount,
		BandCount:     idx.cfg.Bands,
		NumHashes:     idx.cfg.NumHashes,
		ShingleSize:   idx.cfg.ShingleSize,
	}
}
