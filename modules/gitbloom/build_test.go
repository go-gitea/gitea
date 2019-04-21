package gitbloom

import (
	"testing"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/cache"
	"gopkg.in/src-d/go-git.v4/plumbing/filemode"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
	"gopkg.in/src-d/go-git.v4/storage/filesystem"

	"gopkg.in/src-d/go-billy.v4/osfs"
)

// Example how to resolve a revision into its commit counterpart
func TestWrite(t *testing.T) {
	path := "C:\\Users\\Filip Navara\\gitea-repositories\\filip\\linux.git"

	// We instantiate a new repository targeting the given path (the .git folder)
	fs := osfs.New(path)
	s := filesystem.NewStorageWithOptions(fs, cache.NewObjectLRUDefault(), filesystem.Options{KeepDescriptors: true})
	r, _ := git.Open(s, fs)

	memoryIndex := NewMemoryIndex()

	iter, _ := r.CommitObjects()

	iter.ForEach(func(c *object.Commit) error {
		changes := make([]string, 0, 512)
		aTree, _ := c.Tree()
		if c.NumParents() > 0 {
			c.Parents().ForEach(func(parent *object.Commit) error {
				bTree, _ := parent.Tree()
				changes = updateBloomFilter(changes, aTree, bTree, "")
				return storer.ErrStop
			})
			if len(changes) < 512 {
				bloomFilter := NewBloomPathFilter(len(changes))
				for _, change := range changes {
					bloomFilter.Add(change)
				}
				memoryIndex.Add(c.ID(), bloomFilter)
			}
		}
		return nil
	})

	f, _ := fs.Create("bloom")
	e := NewEncoder(f)
	e.Encode(memoryIndex)
	f.Close()
}

func getFullPath(treePath, path string) string {
	if treePath != "" {
		if path != "" {
			return treePath + "/" + path
		}
		return treePath
	}
	return path
}

func dumpTreeIntoBloomFilter(changes []string, tree *object.Tree, treePath string) []string {
	for _, entry := range tree.Entries {
		fullPath := getFullPath(treePath, entry.Name)
		changes = append(changes, fullPath)
		if entry.Mode == filemode.Dir {
			if subtree, err := tree.Tree(entry.Name); err == nil {
				dumpTreeIntoBloomFilter(changes, subtree, fullPath)
			}
		}
	}
	return changes
}

func updateBloomFilter(changes []string, a, b *object.Tree, treePath string) []string {
	aHashes := make(map[string]plumbing.Hash)
	for _, entry := range a.Entries {
		aHashes[entry.Name] = entry.Hash
	}

	for _, entry := range b.Entries {
		if aHashes[entry.Name] != entry.Hash {
			// File from 'b' didn't exist in 'a', or it has different hash than in 'a'
			fullPath := getFullPath(treePath, entry.Name)
			if entry.Mode == filemode.Dir {
				aTree, _ := a.Tree(entry.Name)
				bTree, _ := b.Tree(entry.Name)
				if aTree != nil && bTree != nil {
					changes = updateBloomFilter(changes, aTree, bTree, fullPath)
					changes = append(changes, fullPath)
				} else if aTree != nil {
					changes = dumpTreeIntoBloomFilter(changes, aTree, fullPath)
				} else if bTree != nil {
					changes = dumpTreeIntoBloomFilter(changes, bTree, fullPath)
				}
			} else {
				changes = append(changes, fullPath)
			}
		}
		delete(aHashes, entry.Name)
	}

	for name := range aHashes {
		// File from 'a' is removed in 'b'
		changes = append(changes, getFullPath(treePath, name))
	}

	return changes
}

func createBloomFilter(a, b *object.Tree) *BloomPathFilter {
	changes := make([]string, 0, 512)
	changes = updateBloomFilter(changes, a, b, "")
	bloomFilter := NewBloomPathFilter(len(changes))
	for _, change := range changes {
		bloomFilter.Add(change)
	}
	return bloomFilter
}
