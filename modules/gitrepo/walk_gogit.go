// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package gitrepo

import (
	"context"

	"github.com/go-git/go-git/v5/plumbing"
)

// WalkReferences walks all the references from the repository
// refname is empty, ObjectTag or ObjectBranch. All other values should be treated as equivalent to empty.
func WalkReferences(ctx context.Context, repo Repository, walkfn func(sha1, refname string) error) (int, error) {
	gitRepo := repositoryFromContext(ctx, repo)
	if gitRepo == nil {
		var err error
		gitRepo, err = OpenRepository(ctx, repo)
		if err != nil {
			return 0, err
		}
		defer gitRepo.Close()
	}

	i := 0
	iter, err := gitRepo.GoGitRepo().References()
	if err != nil {
		return i, err
	}
	defer iter.Close()

	err = iter.ForEach(func(ref *plumbing.Reference) error {
		err := walkfn(ref.Hash().String(), string(ref.Name()))
		i++
		return err
	})
	return i, err
}
