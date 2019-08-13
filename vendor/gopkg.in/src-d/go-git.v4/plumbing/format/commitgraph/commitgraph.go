package commitgraph

import (
	"time"

	"gopkg.in/src-d/go-git.v4/plumbing"
)

// CommitData is a reduced representation of Commit as presented in the commit graph
// file. It is merely useful as an optimization for walking the commit graphs.
type CommitData struct {
	// TreeHash is the hash of the root tree of the commit.
	TreeHash plumbing.Hash
	// ParentIndexes are the indexes of the parent commits of the commit.
	ParentIndexes []int
	// ParentHashes are the hashes of the parent commits of the commit.
	ParentHashes []plumbing.Hash
	// Generation number is the pre-computed generation in the commit graph
	// or zero if not available
	Generation int
	// When is the timestamp of the commit.
	When time.Time
}

// Index represents a representation of commit graph that allows indexed
// access to the nodes using commit object hash
type Index interface {
	// GetIndexByHash gets the index in the commit graph from commit hash, if available
	GetIndexByHash(h plumbing.Hash) (int, error)
	// GetNodeByIndex gets the commit node from the commit graph using index
	// obtained from child node, if available
	GetCommitDataByIndex(i int) (*CommitData, error)
	// Hashes returns all the hashes that are available in the index
	Hashes() []plumbing.Hash
}
