package gitbloom

import (
	"gopkg.in/src-d/go-git.v4/plumbing"
)

// Index provides methods to access bloom filters individual commits
type Index interface {
	// GetBloomByHash gets the bloom path filter for particular commit
	GetBloomByHash(h plumbing.Hash) (*BloomPathFilter, error)
	// Hashes returns all the hashes that are available in the index
	Hashes() []plumbing.Hash
}
