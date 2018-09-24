// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
)

// GetCommitsInfo gets information of all commits that are corresponding to these entries
func (tes Entries) GetCommitsInfo(commit *Commit, treePath string, cache LastCommitCache) ([][]interface{}, *Commit, error) {
	entryPaths := make([]string, len(tes))
	for i, entry := range tes {
		entryPaths[i] = entry.Name()
	}

	c, err := commit.repo.gogitRepo.CommitObject(plumbing.Hash(commit.ID))
	if err != nil {
		return nil, nil, err
	}

	revs, treeCommit, err := getLastCommitForPaths(c, treePath, entryPaths)
	if err != nil {
		return nil, nil, err
	}

	commit.repo.gogitStorage.Close()

	commitsInfo := make([][]interface{}, len(tes))
	for i, entry := range tes {
		commit := &Commit{
			ID:            revs[i].Hash,
			CommitMessage: revs[i].Message,
			Committer: &Signature{
				When: revs[i].Committer.When,
			},
		}
		commitsInfo[i] = []interface{}{entry, commit}
	}
	return commitsInfo, convertCommit(treeCommit), nil
}

func convertCommit(c *object.Commit) *Commit {
	var pgpSignaure *CommitGPGSignature
	if c.PGPSignature != "" {
		pgpSignaure = &CommitGPGSignature{
			Signature: c.PGPSignature,
			Payload:   c.Message, // FIXME: This is not correct
		}
	}

	return &Commit{
		ID:            c.Hash,
		CommitMessage: c.Message,
		Committer:     &c.Committer,
		Author:        &c.Author,
		Signature:     pgpSignaure,
		parents:       c.ParentHashes,
	}
}

func getLastCommitForPaths(c *object.Commit, treePath string, paths []string) ([]*object.Commit, *object.Commit, error) {
	cIter := object.NewCommitIterCTime(c, nil, nil)
	result := make([]*object.Commit, len(paths))
	var resultTree *object.Commit
	remainingResults := len(paths)

	cTree, err := c.Tree()
	if err != nil {
		return nil, nil, err
	}

	if treePath != "" {
		cTree, err = cTree.Tree(treePath)
		if err != nil {
			return nil, nil, err
		}
	}
	lastTreeHash := cTree.Hash

	currentEntryHashes := make([]plumbing.Hash, len(paths))
	for i, path := range paths {
		cEntry, err := cTree.FindEntry(path)
		if err != nil {
			return nil, nil, err
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

			if treePath != "" {
				parentTree, err = parentTree.Tree(treePath)
				// the whole tree doesn't exist
				if err != nil {
					if resultTree == nil {
						resultTree = current
					}
					return nil
				}
			}

			// bail-out early if this tree branch was not changed in the commit
			if lastTreeHash == parentTree.Hash {
				copy(newEntryHashes, currentEntryHashes)
				return nil
			} else if resultTree == nil {
				// save the latest commit that updated treePath
				resultTree = current
			}
			lastTreeHash = parentTree.Hash

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

	return result, resultTree, nil
}
