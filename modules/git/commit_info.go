// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"path"

	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
)

// GetCommitsInfo gets information of all commits that are corresponding to these entries
func (tes Entries) GetCommitsInfo(commit *Commit, treePath string, cache LastCommitCache) ([][]interface{}, error) {
	entryPaths := make([]string, len(tes))
	for i, entry := range tes {
		entryPaths[i] = path.Join(treePath, entry.Name())
	}

	c, err := commit.repo.gogitRepo.CommitObject(plumbing.Hash(commit.ID))
	if err != nil {
		return nil, err
	}

	revs, err := getLastCommitForPaths(c, entryPaths)
	if err != nil {
		return nil, err
	}

	commit.repo.gogitStorage.Close()

	commitsInfo := make([][]interface{}, len(tes))
	for i, entry := range tes {
		commit := &Commit{
			ID:            SHA1(revs[i].Hash),
			CommitMessage: revs[i].Message,
			Committer: &Signature{
				When: revs[i].Committer.When,
			},
		}
		commitsInfo[i] = []interface{}{entry, commit}
	}
	return commitsInfo, nil
}

func getLastCommitForPaths(c *object.Commit, paths []string) ([]*object.Commit, error) {
	cIter := object.NewCommitIterCTime(c, nil, nil)
	result := make([]*object.Commit, len(paths))
	remainingResults := len(paths)

	cTree, err := c.Tree()
	if err != nil {
		return nil, err
	}

	currentEntryHashes := make([]plumbing.Hash, len(paths))
	for i, path := range paths {
		cEntry, err := cTree.FindEntry(path)
		if err != nil {
			return nil, err
		}
		currentEntryHashes[i] = cEntry.Hash
	}

	cIter.ForEach(func(current *object.Commit) error {
		newEntryHashes := make([]plumbing.Hash, len(paths))

		err := current.Parents().ForEach(func(parent *object.Commit) error {
			parentTree, err := parent.Tree()
			if err != nil {
				return err
			}

			for i, path := range paths {
				// skip path if we already found it
				if currentEntryHashes[i] != plumbing.ZeroHash {
					// find parents that contain the path
					if parentEntry, err := parentTree.FindEntry(path); err == nil {
						// if the hash for the path differs in the parent then the current commit changed it
						if parentEntry.Hash == currentEntryHashes[i] {
							newEntryHashes[i] = currentEntryHashes[i]
						} else {
							// mark for saving the result below
							newEntryHashes[i] = plumbing.ZeroHash
							// stop any further processing for this file
							currentEntryHashes[i] = plumbing.ZeroHash
						}
					}
				}
			}

			return nil
		})
		if err != nil {
			return err
		}

		// if a file didn't exist in any parent commit then it must have been created in the
		// current one. also we mark changed files in the loop above as not present in the
		// parent to simplify processing
		for i, newEntryHash := range newEntryHashes {
			if newEntryHash == plumbing.ZeroHash && result[i] == nil {
				result[i] = current
				remainingResults--
			}
		}

		if remainingResults == 0 {
			return storer.ErrStop
		}

		currentEntryHashes = newEntryHashes
		return nil
	})

	return result, nil
}
