// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"

	"gitea.dev/modules/git"
)

// CacheRef caches last commit information of the branch or the tag
func CacheRef(ctx context.Context, gitRepo *git.Repository, fullRefName git.RefName) error {
	commit, err := gitRepo.GetCommit(ctx, fullRefName.String())
	if err != nil {
		return err
	}
	return commit.CacheCommit(ctx, gitRepo)
}
