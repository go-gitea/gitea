// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"context"
	"path"

	"github.com/emirpasic/gods/trees/binaryheap"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	cgobject "github.com/go-git/go-git/v5/plumbing/object/commitgraph"
)

// GetCommitsInfo gets information of all commits that are corresponding to these entries
func (tes Entries) GetCommitsInfo(ctx context.Context, commit *Commit, treePath string) ([]CommitInfo, *Commit, error) {
	entryPaths := make([]string, len(tes)+1)
	// Get the commit for the treePath itself
	entryPaths[0] = ""
	for i, entry := range tes {
		entryPaths[i+1] = entry.Name()
	}

	commitNodeIndex, commitGraphFile := commit.repo.CommitNodeIndex()
	if commitGraphFile != nil {
		defer commitGraphFile.Close()
	}

	c, err := commitNodeIndex.Get(plumbing.Hash(commit.ID.RawValue()))
	if err != nil {
		return nil, nil, err
	}

	var revs map[string]*Commit
	if commit.repo.LastCommitCache != nil {
		var unHitPaths []string
		revs, unHitPaths, err = getLastCommitForPathsByCache(commit.ID.String(), treePath, entryPaths, commit.repo.LastCommitCache)
		if err != nil {
			return nil, nil, err
		}
		if len(unHitPaths) > 0 {
			revs2, err := GetLastCommitForPaths(ctx, commit.repo.LastCommitCache, c, treePath, unHitPaths)
			if err != nil {
				return nil, nil, err
			}

			for k, v := range revs2 {
				revs[k] = v
			}
		}
	} else {
		revs, err = GetLastCommitForPaths(ctx, nil, c, treePath, entryPaths)
	}
	if err != nil {
		return nil, nil, err
	}

	commit.repo.gogitStorage.Close()

	commitsInfo := make([]CommitInfo, len(tes))
	for i, entry := range tes {
		commitsInfo[i] = CommitInfo{
			Entry: entry,
		}

		// Check if we have found a commit for this entry in time
		if entryCommit, ok := revs[entry.Name()]; ok {
			commitsInfo[i].Commit = entryCommit
		}

		// If the entry is a submodule add a submodule file for this
		if entry.IsSubModule() {
			subModuleURL := ""
			var fullPath string
			if len(treePath) > 0 {
				fullPath = treePath + "/" + entry.Name()
			} else {
				fullPath = entry.Name()
			}
			if subModule, err := commit.GetSubModule(fullPath); err != nil {
				return nil, nil, err
			} else if subModule != nil {
				subModuleURL = subModule.URL
			}
			subModuleFile := NewCommitSubmoduleFile(subModuleURL, entry.ID.String())
			commitsInfo[i].SubmoduleFile = subModuleFile
		}
	}

	// Retrieve the commit for the treePath itself (see above). We basically
	// get it for free during the tree traversal and it's used for listing
	// pages to display information about newest commit for a given path.
	var treeCommit *Commit
	var ok bool
	if treePath == "" {
		treeCommit = commit
	} else if treeCommit, ok = revs[""]; ok {
		treeCommit.repo = commit.repo
	}
	return commitsInfo, treeCommit, nil
}

type commitAndPaths struct {
	commit cgobject.CommitNode
	// Paths that are still on the branch represented by commit
	paths []string
	// Set of hashes for the paths
	hashes map[string]plumbing.Hash
}

func getCommitTree(c cgobject.CommitNode, treePath string) (*object.Tree, error) {
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

func getFileHashes(c cgobject.CommitNode, treePath string, paths []string) (map[string]plumbing.Hash, error) {
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

func getLastCommitForPathsByCache(commitID, treePath string, paths []string, cache *LastCommitCache) (map[string]*Commit, []string, error) {
	var unHitEntryPaths []string
	results := make(map[string]*Commit)
	for _, p := range paths {
		lastCommit, err := cache.Get(commitID, path.Join(treePath, p))
		if err != nil {
			return nil, nil, err
		}
		if lastCommit != nil {
			results[p] = lastCommit
			continue
		}

		unHitEntryPaths = append(unHitEntryPaths, p)
	}

	return results, unHitEntryPaths, nil
}

// GetLastCommitForPaths returns last commit information
func GetLastCommitForPaths(ctx context.Context, cache *LastCommitCache, c cgobject.CommitNode, treePath string, paths []string) (map[string]*Commit, error) {
	refSha := c.ID().String()

	// We do a tree traversal with nodes sorted by commit time
	heap := binaryheap.NewWith(func(a, b any) int {
		if a.(*commitAndPaths).commit.CommitTime().Before(b.(*commitAndPaths).commit.CommitTime()) {
			return 1
		}
		return -1
	})

	resultNodes := make(map[string]cgobject.CommitNode)
	initialHashes, err := getFileHashes(c, treePath, paths)
	if err != nil {
		return nil, err
	}

	// Start search from the root commit and with full set of paths
	heap.Push(&commitAndPaths{c, paths, initialHashes})
heaploop:
	for {
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				break heaploop
			}
			return nil, ctx.Err()
		default:
		}
		cIn, ok := heap.Pop()
		if !ok {
			break
		}
		current := cIn.(*commitAndPaths)

		// Load the parent commits for the one we are currently examining
		numParents := current.commit.NumParents()
		var parents []cgobject.CommitNode
		for i := 0; i < numParents; i++ {
			parent, err := current.commit.ParentNode(i)
			if err != nil {
				break
			}
			parents = append(parents, parent)
		}

		// Examine the current commit and set of interesting paths
		pathUnchanged := make([]bool, len(current.paths))
		parentHashes := make([]map[string]plumbing.Hash, len(parents))
		for j, parent := range parents {
			parentHashes[j], err = getFileHashes(parent, treePath, current.paths)
			if err != nil {
				break
			}

			for i, path := range current.paths {
				if parentHashes[j][path] == current.hashes[path] {
					pathUnchanged[i] = true
				}
			}
		}

		var remainingPaths []string
		for i, pth := range current.paths {
			// The results could already contain some newer change for the same path,
			// so don't override that and bail out on the file early.
			if resultNodes[pth] == nil {
				if pathUnchanged[i] {
					// The path existed with the same hash in at least one parent so it could
					// not have been changed in this commit directly.
					remainingPaths = append(remainingPaths, pth)
				} else {
					// There are few possible cases how can we get here:
					// - The path didn't exist in any parent, so it must have been created by
					//   this commit.
					// - The path did exist in the parent commit, but the hash of the file has
					//   changed.
					// - We are looking at a merge commit and the hash of the file doesn't
					//   match any of the hashes being merged. This is more common for directories,
					//   but it can also happen if a file is changed through conflict resolution.
					resultNodes[pth] = current.commit
					if err := cache.Put(refSha, path.Join(treePath, pth), current.commit.ID().String()); err != nil {
						return nil, err
					}
				}
			}
		}

		if len(remainingPaths) > 0 {
			// Add the parent nodes along with remaining paths to the heap for further
			// processing.
			for j, parent := range parents {
				// Combine remainingPath with paths available on the parent branch
				// and make union of them
				remainingPathsForParent := make([]string, 0, len(remainingPaths))
				newRemainingPaths := make([]string, 0, len(remainingPaths))
				for _, path := range remainingPaths {
					if parentHashes[j][path] == current.hashes[path] {
						remainingPathsForParent = append(remainingPathsForParent, path)
					} else {
						newRemainingPaths = append(newRemainingPaths, path)
					}
				}

				if remainingPathsForParent != nil {
					heap.Push(&commitAndPaths{parent, remainingPathsForParent, parentHashes[j]})
				}

				if len(newRemainingPaths) == 0 {
					break
				} else {
					remainingPaths = newRemainingPaths
				}
			}
		}
	}

	// Post-processing
	result := make(map[string]*Commit)
	for path, commitNode := range resultNodes {
		commit, err := commitNode.Commit()
		if err != nil {
			return nil, err
		}
		result[path] = convertCommit(commit)
	}

	return result, nil
}
