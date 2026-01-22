// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package pipeline

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"code.gitea.io/gitea/modules/git"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// FindLFSFile finds commits that contain a provided pointer file hash
func FindLFSFile(repo *git.Repository, objectID git.ObjectID) ([]*LFSResult, error) {
	resultsMap := map[string]*LFSResult{}
	results := make([]*LFSResult, 0)

	gogitRepo := repo.GoGitRepo()

	commitsIter, err := gogitRepo.Log(&gogit.LogOptions{
		Order: gogit.LogOrderCommitterTime,
		All:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("LFS error occurred, failed to get GoGit CommitsIter: err: %w", err)
	}

	err = commitsIter.ForEach(func(gitCommit *object.Commit) error {
		tree, err := gitCommit.Tree()
		if err != nil {
			return err
		}
		treeWalker := object.NewTreeWalker(tree, true, nil)
		defer treeWalker.Close()
		for {
			name, entry, err := treeWalker.Next()
			if err == io.EOF {
				break
			}
			if entry.Hash == plumbing.Hash(objectID.RawValue()) {
				parents := make([]git.ObjectID, len(gitCommit.ParentHashes))
				for i, parentCommitID := range gitCommit.ParentHashes {
					parents[i] = git.ParseGogitHash(parentCommitID)
				}

				result := LFSResult{
					Name:         name,
					SHA:          gitCommit.Hash.String(),
					Summary:      strings.Split(strings.TrimSpace(gitCommit.Message), "\n")[0],
					When:         gitCommit.Author.When,
					ParentHashes: parents,
				}
				resultsMap[gitCommit.Hash.String()+":"+name] = &result
			}
		}
		return nil
	})
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("LFS error occurred, failure in CommitIter.ForEach: %w", err)
	}

	for _, result := range resultsMap {
		hasParent := false
		for _, parentHash := range result.ParentHashes {
			if _, hasParent = resultsMap[parentHash.String()+":"+result.Name]; hasParent {
				break
			}
		}
		if !hasParent {
			results = append(results, result)
		}
	}

	sort.Sort(lfsResultSlice(results))
	err = fillResultNameRev(repo.Ctx, repo.Path, results)
	return results, err
}
