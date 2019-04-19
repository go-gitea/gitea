// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"github.com/emirpasic/gods/trees/binaryheap"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

// GetCommitsInfo gets information of all commits that are corresponding to these entries
func (tes Entries) GetCommitsInfo(commit *Commit, treePath string, cache LastCommitCache) ([][]interface{}, *Commit, error) {
	entryPaths := make([]string, len(tes)+1)
	// Get the commit for the treePath itself
	entryPaths[0] = ""
	for i, entry := range tes {
		entryPaths[i+1] = entry.Name()
	}

	c, err := commit.repo.gogitRepo.CommitObject(plumbing.Hash(commit.ID))
	if err != nil {
		return nil, nil, err
	}

	revs, err := getLastCommitForPaths(c, treePath, entryPaths)
	if err != nil {
		return nil, nil, err
	}

	commit.repo.gogitStorage.Close()

	commitsInfo := make([][]interface{}, len(tes))
	for i, entry := range tes {
		if rev, ok := revs[entry.Name()]; ok {
			entryCommit := convertCommit(rev)
			if entry.IsSubModule() {
				subModuleURL := ""
				if subModule, err := commit.GetSubModule(entry.Name()); err != nil {
					return nil, nil, err
				} else if subModule != nil {
					subModuleURL = subModule.URL
				}
				subModuleFile := NewSubModuleFile(entryCommit, subModuleURL, entry.ID.String())
				commitsInfo[i] = []interface{}{entry, subModuleFile}
			} else {
				commitsInfo[i] = []interface{}{entry, entryCommit}
			}
		} else {
			commitsInfo[i] = []interface{}{entry, nil}
		}
	}

	// Retrieve the commit for the treePath itself (see above). We basically
	// get it for free during the tree traversal and it's used for listing
	// pages to display information about newest commit for a given path.
	var treeCommit *Commit
	if rev, ok := revs[""]; ok {
		treeCommit = convertCommit(rev)
	}
	return commitsInfo, treeCommit, nil
}

type commitAndPaths struct {
	commit *object.Commit
	// Paths that are still on the branch represented by commit
	paths []string
	// Set of hashes for the paths
	hashes map[string]plumbing.Hash
}

func getCommitTree(c *object.Commit, treePath string) (*object.Tree, error) {
	tree, err := c.Tree()
	if err != nil {
		return nil, err
	}

	// Optimize deep traversals by focusing only on the specific tree
	if treePath != "" {
		tree, err = tree.Tree(treePath)
		if err != nil {
			return nil, err
		}
	}

	return tree, nil
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

func getFileHashes(c *object.Commit, treePath string, paths []string) (map[string]plumbing.Hash, error) {
	tree, err := getCommitTree(c, treePath)
	if err == object.ErrDirectoryNotFound {
		// The whole tree didn't exist, so return empty map
		return make(map[string]plumbing.Hash), nil
	}
	if err != nil {
		return nil, err
	}

	hashes := make(map[string]plumbing.Hash)
	for _, path := range paths {
		if path != "" {
			entry, err := tree.FindEntry(path)
			if err == nil {
				hashes[path] = entry.Hash
			}
		} else {
			hashes[path] = tree.Hash
		}
	}

	return hashes, nil
}

func getLastCommitForPaths(c *object.Commit, treePath string, paths []string) (map[string]*object.Commit, error) {
	// We do a tree traversal with nodes sorted by commit time
	seen := make(map[plumbing.Hash]bool)
	heap := binaryheap.NewWith(func(a, b interface{}) int {
		if a.(*commitAndPaths).commit.Committer.When.Before(b.(*commitAndPaths).commit.Committer.When) {
			return 1
		}
		return -1
	})

	result := make(map[string]*object.Commit)
	initialHashes, err := getFileHashes(c, treePath, paths)
	if err != nil {
		return nil, err
	}

	// Start search from the root commit and with full set of paths
	heap.Push(&commitAndPaths{c, paths, initialHashes})

	for {
		cIn, ok := heap.Pop()
		if !ok {
			break
		}
		current := cIn.(*commitAndPaths)
		currentID := current.commit.ID()

		if seen[currentID] {
			continue
		}
		seen[currentID] = true

		// Load the parent commits for the one we are currently examining
		numParents := current.commit.NumParents()
		var parents []*object.Commit
		for i := 0; i < numParents; i++ {
			parent, err := current.commit.Parent(i)
			if err != nil {
				break
			}
			parents = append(parents, parent)
		}

		// Examine the current commit and set of interesting paths
		numOfParentsWithPath := make([]int, len(current.paths))
		pathChanged := make([]bool, len(current.paths))
		parentHashes := make([]map[string]plumbing.Hash, len(parents))
		for j, parent := range parents {
			parentHashes[j], err = getFileHashes(parent, treePath, current.paths)
			if err != nil {
				break
			}

			for i, path := range current.paths {
				if parentHashes[j][path] != plumbing.ZeroHash {
					numOfParentsWithPath[i]++
					if parentHashes[j][path] != current.hashes[path] {
						pathChanged[i] = true
					}
				}
			}
		}

		var remainingPaths []string
		for i, path := range current.paths {
			switch numOfParentsWithPath[i] {
			case 0:
				// The path didn't exist in any parent, so it must have been created by
				// this commit. The results could already contain some newer change from
				// different path, so don't override that.
				if result[path] == nil {
					result[path] = current.commit
				}
			case 1:
				// The file is present on exactly one parent, so check if it was changed
				// and save the revision if it did.
				if pathChanged[i] {
					if result[path] == nil {
						result[path] = current.commit
					}
				} else {
					remainingPaths = append(remainingPaths, path)
				}
			default:
				// The file is present on more than one of the parent paths, so this is
				// a merge. We have to examine all the parent trees to find out where
				// the change occurred. pathChanged[i] would tell us that the file was
				// changed during the merge, but it wouldn't tell us the relevant commit
				// that introduced it.
				remainingPaths = append(remainingPaths, path)
			}
		}

		if len(remainingPaths) > 0 {
			// Add the parent nodes along with remaining paths to the heap for further
			// processing.
			for j, parent := range parents {
				if seen[parent.ID()] {
					continue
				}

				// Combine remainingPath with paths available on the parent branch
				// and make union of them
				var remainingPathsForParent []string
				for _, path := range remainingPaths {
					if parentHashes[j][path] != plumbing.ZeroHash {
						remainingPathsForParent = append(remainingPathsForParent, path)
					}
				}

				heap.Push(&commitAndPaths{parent, remainingPathsForParent, parentHashes[j]})
			}
		}
	}

	return result, nil
}
