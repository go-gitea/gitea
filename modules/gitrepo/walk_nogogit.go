// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package gitrepo

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
)

// WalkReferences walks all the references from the repository
func WalkReferences(ctx context.Context, repo *repo_model.Repository, walkfn func(sha1, refname string) error) (int, error) {
	return git.WalkShowRef(ctx, repo.RepoPath(), nil, 0, 0, walkfn)
}
