package git

import (
	"io/ioutil"
	"os"
	"path"

	"golang.org/x/exp/mmap"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/commitgraph"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	cgobject "gopkg.in/src-d/go-git.v4/plumbing/object/commitgraph"
)

// CommitNodeIndex returns the index for walking commit graph
func (r *Repository) CommitNodeIndex() (cgobject.CommitNodeIndex, *mmap.ReaderAt) {
	indexPath := path.Join(r.Path, "objects", "info", "commit-graph")

	file, err := mmap.Open(indexPath)
	if err == nil {
		index, err := commitgraph.OpenFileIndex(file)
		if err == nil {
			return cgobject.NewGraphCommitNodeIndex(index, r.gogitRepo.Storer), file
		}
	}

	return cgobject.NewObjectCommitNodeIndex(r.gogitRepo.Storer), nil
}

// BuildCommitGraph builds the commit-graph index file
func (r *Repository) BuildCommitGraph(withBloomFilters bool) error {
	h, err := r.gogitRepo.Head()
	if err != nil {
		return err
	}

	commit, err := r.gogitRepo.CommitObject(h.Hash())
	if err != nil {
		return err
	}

	// TODO: Incremental updates
	idx, err := buildCommitGraph(commit, withBloomFilters)
	if err != nil {
		return err
	}

	f, err := ioutil.TempFile(path.Join(r.Path, "objects", "info"), "commit-graph-tmp")
	if err != nil {
		return err
	}

	tmpName := f.Name()
	encoder := commitgraph.NewEncoder(f)
	err = encoder.Encode(idx)
	f.Close()
	if err == nil {
		indexPath := path.Join(r.Path, "objects", "info", "commit-graph")
		os.Remove(indexPath)
		err = os.Rename(tmpName, indexPath)
		if err == nil {
			return nil
		}
	}
	os.Remove(tmpName)
	return err
}

func buildCommitGraph(c *object.Commit, withBloomFilters bool) (*commitgraph.MemoryIndex, error) {
	idx := commitgraph.NewMemoryIndex()
	seen := make(map[plumbing.Hash]bool)
	// TODO: Unroll the recursion
	return idx, addCommitToIndex(idx, c, seen, withBloomFilters)
}

/*
func dumpTreeIntoBloomFilter(bloomFilter *commitgraph.BloomPathFilter, tree *object.Tree, treePath string) {
	for _, entry := range tree.Entries {
		fullPath := getFullPath(treePath, entry.Name)
		bloomFilter.Add(fullPath)
		if entry.Mode == filemode.Dir {
			if subtree, err := tree.Tree(entry.Name); err == nil {
				dumpTreeIntoBloomFilter(bloomFilter, subtree, fullPath)
			}
		}
	}
}

func updateBloomFilter(bloomFilter *commitgraph.BloomPathFilter, a, b *object.Tree, treePath string) {
	aHashes := make(map[string]plumbing.Hash)
	for _, entry := range a.Entries {
		aHashes[entry.Name] = entry.Hash
	}

	for _, entry := range b.Entries {
		if aHashes[entry.Name] != entry.Hash {
			// File from 'b' didn't exist in 'a', or it has different hash than in 'a'
			fullPath := getFullPath(treePath, entry.Name)
			bloomFilter.Add(fullPath)
			if entry.Mode == filemode.Dir {
				aTree, _ := a.Tree(entry.Name)
				bTree, _ := b.Tree(entry.Name)
				if aTree != nil && bTree != nil {
					updateBloomFilter(bloomFilter, aTree, bTree, fullPath)
				} else if aTree != nil {
					dumpTreeIntoBloomFilter(bloomFilter, aTree, fullPath)
				} else if bTree != nil {
					dumpTreeIntoBloomFilter(bloomFilter, bTree, fullPath)
				}
			}
		}
		delete(aHashes, entry.Name)
	}

	for name := range aHashes {
		// File from 'a' is removed in 'b'
		bloomFilter.Add(getFullPath(treePath, name))
	}
}

func createBloomFilter(a, b *object.Tree) *commitgraph.BloomPathFilter {
	bloomFilter := commitgraph.NewBloomPathFilter()
	updateBloomFilter(bloomFilter, a, b, "")
	return bloomFilter
}*/

func addCommitToIndex(idx *commitgraph.MemoryIndex, c *object.Commit, seen map[plumbing.Hash]bool, withBloomFilters bool) error {
	if seen[c.Hash] {
		return nil
	}
	seen[c.Hash] = true

	// Recursively add parents first
	err := c.Parents().ForEach(func(parent *object.Commit) error {
		return addCommitToIndex(idx, parent, seen, withBloomFilters)
	})
	if err != nil {
		return err
	}

	// Calculate file difference to first parent commit
	/*var bloomFilter *commitgraph.BloomPathFilter
	if withBloomFilters && c.NumParents() == 1 {
		if parent, err := c.Parent(0); err == nil {
			if tree, err := c.Tree(); err == nil {
				if parentTree, err := parent.Tree(); err == nil {
					bloomFilter = createBloomFilter(parentTree, tree)
				}
			}
		}
	}*/

	// Add this commit if it hasn't been done already
	node := &commitgraph.CommitData{
		TreeHash:     c.TreeHash,
		ParentHashes: c.ParentHashes,
		When:         c.Committer.When,
	}
	idx.Add /*WithBloom*/ (c.Hash, node /*, bloomFilter*/)
	return nil
}
