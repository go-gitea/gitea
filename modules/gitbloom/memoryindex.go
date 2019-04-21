package gitbloom

import (
	"gopkg.in/src-d/go-git.v4/plumbing"
)

type MemoryIndex struct {
	filterMap map[plumbing.Hash]*BloomPathFilter
}

// NewMemoryIndex creates in-memory commit graph representation
func NewMemoryIndex() *MemoryIndex {
	return &MemoryIndex{make(map[plumbing.Hash]*BloomPathFilter)}
}

// GetBloomByHash gets the bloom path filter for particular commit
func (mi *MemoryIndex) GetBloomByHash(h plumbing.Hash) (*BloomPathFilter, error) {
	if filter, ok := mi.filterMap[h]; ok {
		return filter, nil
	}

	return nil, plumbing.ErrObjectNotFound
}

// Add adds new filter to the memory index
func (mi *MemoryIndex) Add(hash plumbing.Hash, filter *BloomPathFilter) {
	mi.filterMap[hash] = filter
}

// Hashes returns all the hashes that are available in the index
func (mi *MemoryIndex) Hashes() []plumbing.Hash {
	hashes := make([]plumbing.Hash, 0, len(mi.filterMap))
	for k := range mi.filterMap {
		hashes = append(hashes, k)
	}
	return hashes
}
