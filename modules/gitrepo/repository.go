// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"io"

	"code.gitea.io/gitea/modules/git"
)

type GitRepository interface {
	io.Closer
	GetBranches(skip, limit int) ([]*git.Branch, int, error)
	GetRefCommitID(name string) (string, error)
	IsObjectExist(sha string) bool
	GetBranchCommit(branch string) (*git.Commit, error)
	GetDefaultBranch()
	GetObjectFormat()
	IsBranchExist()
	IsTagExist()
	GetTagCommit()
	GetCommit()
}

// OpenRepository opens the repository at the given relative path with the provided context.
func OpenRepository(ctx context.Context, repo Repository) (GitRepository, error) {
	return curService.OpenRepository(ctx, repoRelativePath(repo))
}

func OpenWikiRepository(ctx context.Context, repo Repository) (GitRepository, error) {
	return curService.OpenRepository(ctx, wikiRelativePath(repo))
}
