// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"bufio"
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
	GetDefaultBranch() (string, error)
	GetObjectFormat() (git.ObjectFormat, error)
	IsReferenceExist(string) (bool, error)
	GetCommit(string) (*git.Commit, error)
	GetRelativePath() string
	CatFileBatch(ctx context.Context) (git.WriteCloserError, *bufio.Reader, func())
	CatFileBatchCheck(ctx context.Context) (git.WriteCloserError, *bufio.Reader, func())
}

// OpenRepository opens the repository at the given relative path with the provided context.
func OpenRepository(ctx context.Context, repo Repository) (GitRepository, error) {
	return curService.OpenRepository(ctx, repoRelativePath(repo))
}

func OpenWikiRepository(ctx context.Context, repo Repository) (GitRepository, error) {
	return curService.OpenRepository(ctx, wikiRelativePath(repo))
}
