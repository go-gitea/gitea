// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/reqctx"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// Repository represents a git repository which stored in a disk
type Repository interface {
	RelativePath() string // We don't assume how the directory structure of the repository is, so we only need the relative path
}

// RelativePath should be an unix style path like username/reponame.git
// This method should change it according to the current OS.
func repoPath(repo Repository) string {
	return filepath.Join(setting.RepoRootPath, filepath.FromSlash(repo.RelativePath()))
}

// OpenRepository opens the repository at the given relative path with the provided context.
func OpenRepository(ctx context.Context, repo Repository) (*git.Repository, error) {
	return git.OpenRepository(ctx, repoPath(repo))
}

// contextKey is a value for use with context.WithValue.
type contextKey struct {
	repoPath string
}

// RepositoryFromContextOrOpen attempts to get the repository from the context or just opens it
// The caller must call "defer gitRepo.Close()"
func RepositoryFromContextOrOpen(ctx context.Context, repo Repository) (*git.Repository, io.Closer, error) {
	reqCtx := reqctx.FromContext(ctx)
	if reqCtx != nil {
		gitRepo, err := RepositoryFromRequestContextOrOpen(reqCtx, repo)
		return gitRepo, util.NopCloser{}, err
	}
	gitRepo, err := OpenRepository(ctx, repo)
	return gitRepo, gitRepo, err
}

// RepositoryFromRequestContextOrOpen opens the repository at the given relative path in the provided request context.
// Caller shouldn't close the git repo manually, the git repo will be automatically closed when the request context is done.
func RepositoryFromRequestContextOrOpen(ctx reqctx.RequestContext, repo Repository) (*git.Repository, error) {
	ck := contextKey{repoPath: repoPath(repo)}
	if gitRepo, ok := ctx.Value(ck).(*git.Repository); ok {
		return gitRepo, nil
	}
	gitRepo, err := git.OpenRepository(ctx, ck.repoPath)
	if err != nil {
		return nil, err
	}
	ctx.AddCloser(gitRepo)
	ctx.SetContextValue(ck, gitRepo)
	return gitRepo, nil
}

// IsRepositoryExist returns true if the repository directory exists in the disk
func IsRepositoryExist(ctx context.Context, repo Repository) (bool, error) {
	return util.IsExist(repoPath(repo))
}

// DeleteRepository deletes the repository directory from the disk
func DeleteRepository(ctx context.Context, repo Repository) error {
	return util.RemoveAll(repoPath(repo))
}

// RenameRepository renames a repository's name on disk
func RenameRepository(ctx context.Context, repo, newRepo Repository) error {
	if err := util.Rename(repoPath(repo), repoPath(newRepo)); err != nil {
		return fmt.Errorf("rename repository directory: %w", err)
	}
	return nil
}
