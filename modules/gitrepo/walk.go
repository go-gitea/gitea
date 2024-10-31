// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"

	"code.gitea.io/gitea/modules/git"
)

// WalkReferences walks all the references from the repository
func WalkReferences(ctx context.Context, repo Repository, refType git.ObjectType, walkfn func(sha1, refname string) error) (int, error) {
	return curService.WalkReferences(ctx, repo, refType, 0, 0, walkfn)
}
