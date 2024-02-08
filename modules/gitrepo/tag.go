// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git"
)

// IsTagExist returns true if given tag exists in the repository.
func IsTagExist(ctx context.Context, repo Repository, name string) bool {
	return IsReferenceExist(ctx, repo, git.TagPrefix+name)
}
