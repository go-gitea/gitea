// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package gitrepo

import (
	"context"
)

// WalkReferences walks all the references from the repository
func WalkReferences(ctx context.Context, repo Repository, walkfn func(sha1, refname string) error) (int, error) {
	return curService.WalkReferences(ctx, repoRelativePath(repo), walkfn)
}
