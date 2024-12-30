// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"io"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/reqctx"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

type Repository interface {
	GetName() string
	GetOwnerName() string
}

func repoPath(repo Repository) string {
	return filepath.Join(setting.RepoRootPath, strings.ToLower(repo.GetOwnerName()), strings.ToLower(repo.GetName())+".git")
}

func wikiPath(repo Repository) string {
	return filepath.Join(setting.RepoRootPath, strings.ToLower(repo.GetOwnerName()), strings.ToLower(repo.GetName())+".wiki.git")
}

// OpenRepository opens the repository at the given relative path with the provided context.
func OpenRepository(ctx context.Context, repo Repository) (*git.Repository, error) {
	return git.OpenRepository(ctx, repoPath(repo))
}

func OpenWikiRepository(ctx context.Context, repo Repository) (*git.Repository, error) {
	return git.OpenRepository(ctx, wikiPath(repo))
}

// contextKey is a value for use with context.WithValue.
type contextKey struct {
	repoPath string
}

// RepositoryFromContextOrOpen attempts to get the repository from the context or just opens it
func RepositoryFromContextOrOpen(ctx context.Context, repo Repository) (*git.Repository, io.Closer, error) {
	ds := reqctx.GetRequestDataStore(ctx)
	if ds != nil {
		gitRepo, err := RepositoryFromRequestContextOrOpen(ctx, ds, repo)
		return gitRepo, util.NopCloser{}, err
	}
	gitRepo, err := OpenRepository(ctx, repo)
	return gitRepo, gitRepo, err
}

// RepositoryFromRequestContextOrOpen opens the repository at the given relative path in the provided request context
// The repo will be automatically closed when the request context is done
func RepositoryFromRequestContextOrOpen(ctx context.Context, ds reqctx.RequestDataStore, repo Repository) (*git.Repository, error) {
	ck := contextKey{repoPath: repoPath(repo)}
	if gitRepo, ok := ctx.Value(ck).(*git.Repository); ok {
		return gitRepo, nil
	}
	gitRepo, err := git.OpenRepository(ctx, ck.repoPath)
	if err != nil {
		return nil, err
	}
	ds.AddCloser(gitRepo)
	ds.SetContextValue(ck, gitRepo)
	return gitRepo, nil
}
