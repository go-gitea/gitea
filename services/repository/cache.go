// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"

	"code.gitea.io/gitea/modules/git"
)

// CacheRef cache last commit information of the branch or the tag
func CacheRef(ctx context.Context, gitRepo *git.Repository, fullRefName git.RefName) error {
	commit, err := gitRepo.GetCommit(fullRefName.String())
	if err != nil {
		return err
	}

	return gitRepo.CacheCommit(ctx, commit)
}
