// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package gitrepo

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	"github.com/go-git/go-git/v5/plumbing"
)

// WalkReferences walks all the references from the repository
// refType should be empty, ObjectTag or ObjectBranch. All other values are equivalent to empty.
func WalkReferences(ctx context.Context, repo *repo_model.Repository, walkfn func(sha1, refname string) error) (int, error) {
	repo := RepositoryFromContext(ctx, repo)
	if repo == nil {
		var err error
		repo, err = OpenRepository(ctx, repo)
		if err != nil {
			return 0, err
		}
		defer repo.Close()
	}

	i := 0
	iter, err := repo.gogitRepo.References()
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
