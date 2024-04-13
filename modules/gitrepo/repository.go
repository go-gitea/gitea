// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"bufio"
	"context"
	"io"
	"time"

	"code.gitea.io/gitea/modules/git"
)

type GitRepository interface {
	io.Closer
	GetBranches(skip, limit int) ([]*git.Branch, int, error)
	GetRefCommitID(name string) (string, error)
	IsObjectExist(sha string) bool
	GetBranchCommit(branch string) (*git.Commit, error)
	GetObjectFormat() (git.ObjectFormat, error)
	IsReferenceExist(string) (bool, error)
	GetCommit(string) (*git.Commit, error)
	GetRelativePath() string
	CatFileBatch(ctx context.Context) (git.WriteCloserError, *bufio.Reader, func())
	CatFileBatchCheck(ctx context.Context) (git.WriteCloserError, *bufio.Reader, func())
	GetCommitsFromIDs(commitIDs []string) []*git.Commit
	CreateBundle(ctx context.Context, commit string, out io.Writer) error
	CreateArchive(ctx context.Context, format git.ArchiveType, target io.Writer, usePrefix bool, commitID string) error
	GetCodeActivityStats(fromTime time.Time, branch string) (*git.CodeActivityStats, error)
	GetLanguageStats(commitID string) (map[string]int64, error)
	GetBranchNames(skip, limit int) ([]string, int, error)
	GetTagInfos(page, pageSize int) ([]*git.Tag, int, error)
	GetTagCommitID(name string) (string, error)
	WalkReferences(arg git.ObjectType, skip, limit int, walkfn func(sha1, refname string) error) (int, error)
	GetTagWithID(idStr, name string) (*git.Tag, error)
	GetCommitByObjectID(id git.ObjectID) (*git.Commit, error)
}

// OpenRepository opens the repository at the given relative path with the provided context.
func OpenRepository(ctx context.Context, repo Repository) (GitRepository, error) {
	return curService.OpenRepository(ctx, repoRelativePath(repo))
}

func OpenWikiRepository(ctx context.Context, repo Repository) (GitRepository, error) {
	return curService.OpenRepository(ctx, wikiRelativePath(repo))
}
