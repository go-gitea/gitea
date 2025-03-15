// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"fmt"
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

func absPath(owner, name string) string {
	return filepath.Join(setting.RepoRootPath, strings.ToLower(owner), strings.ToLower(name)+".git")
}

func repoPath(repo Repository) string {
	return absPath(repo.GetOwnerName(), repo.GetName())
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
func RenameRepository(ctx context.Context, repo Repository, newName string) error {
	newRepoPath := absPath(repo.GetOwnerName(), newName)
	if err := util.Rename(repoPath(repo), newRepoPath); err != nil {
		return fmt.Errorf("rename repository directory: %w", err)
	}
	return nil
}
