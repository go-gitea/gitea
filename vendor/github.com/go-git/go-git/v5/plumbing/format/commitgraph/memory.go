package commitgraph

import (
	"github.com/go-git/go-git/v5/plumbing"
)

// MemoryIndex provides a way to build the commit-graph in memory
// for later encoding to file.
type MemoryIndex struct {
	commitData []*CommitData
	indexMap   map[plumbing.Hash]int
}

// NewMemoryIndex creates in-memory commit graph representation
func NewMemoryIndex() *MemoryIndex {
	return &MemoryIndex{
		indexMap: make(map[plumbing.Hash]int),
	}
}

// GetIndexByHash gets the index in the commit graph from commit hash, if available
func (mi *MemoryIndex) GetIndexByHash(h plumbing.Hash) (int, error) {
	i, ok := mi.indexMap[h]
	if ok {
		return i, nil
	}

	return 0, plumbing.ErrObjectNotFound
}

// GetCommitDataByIndex gets the commit node from the commit graph using index
// obtained from child node, if available
func (mi *MemoryIndex) GetCommitDataByIndex(i int) (*CommitData, error) {
	if i >= len(mi.commitData) {
		return nil, plumbing.ErrObjectNotFound
	}

	commitData := mi.commitData[i]

	// Map parent hashes to parent indexes
	if commitData.ParentIndexes == nil {
		parentIndexes := make([]int, len(commitData.ParentHashes))
		for i, parentHash := range commitData.ParentHashes {
			var err error
			if parentIndexes[i], err = mi.GetIndexByHash(parentHash); err != nil {
				return nil, err
			}
		}
		commitData.ParentIndexes = parentIndexes
	}

	return commitData, nil
}

// Hashes returns all the hashes that are available in the index
func (mi *MemoryIndex) Hashes() []plumbing.Hash {
	hashes := make([]plumbing.Hash, 0, len(mi.indexMap))
	for k := range mi.indexMap {
		hashes = append(hashes, k)
	}
	return hashes
}

// Add adds new node to the memory index
func (mi *MemoryIndex) Add(hash plumbing.Hash, commitData *CommitData) {
	// The parent indexes are calculated lazily in GetNodeByIndex
	// which allows adding nodes out of order as long as all parents
	// are eventually resolved
	commitData.ParentIndexes = nil
	mi.indexMap[hash] = len(mi.commitData)
	mi.commitData = append(mi.commitData, commitData)
}
