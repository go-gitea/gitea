// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"fmt"
	"io"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/util"
)

type Repository = git.Repository

// OpenRepository opens the repository at the given relative path with the provided context.
func OpenRepository(ctx context.Context, repo *repo_model.Repository) (*Repository, error) {
	return git.OpenRepository(ctx, repo.RepoPath())
}

func OpenWikiRepository(ctx context.Context, repo *repo_model.Repository) (*Repository, error) {
	return git.OpenRepository(ctx, repo.WikiPath())
}

// DeleteRepository deletes the repository at the given relative path with the provided context.
func DeleteRepository(ctx context.Context, repo *repo_model.Repository) error {
	if err := util.RemoveAll(repo.RepoPath()); err != nil {
		return fmt.Errorf("failed to remove %s: %w", repo.FullName(), err)
	}
	return nil
}

// contextKey is a value for use with context.WithValue.
type contextKey struct {
	name string
}

// RepositoryContextKey is a context key. It is used with context.Value() to get the current Repository for the context
var RepositoryContextKey = &contextKey{"repository"}

// RepositoryFromContext attempts to get the repository from the context
func RepositoryFromContext(ctx context.Context, repo *repo_model.Repository) *Repository {
	value := ctx.Value(RepositoryContextKey)
	if value == nil {
		return nil
	}

	if gitRepo, ok := value.(*Repository); ok {
		if gitRepo.Path == repo.RepoPath() {
			return gitRepo
		}
	}

	return nil
}

type nopCloser func()

func (nopCloser) Close() error { return nil }

// RepositoryFromContextOrOpen attempts to get the repository from the context or just opens it
func RepositoryFromContextOrOpen(ctx context.Context, repo *repo_model.Repository) (*Repository, io.Closer, error) {
	gitRepo := RepositoryFromContext(ctx, repo)
	if gitRepo != nil {
		return gitRepo, nopCloser(nil), nil
	}

	gitRepo, err := OpenRepository(ctx, repo)
	return gitRepo, gitRepo, err
}

// RepositoryFromContext attempts to get the repository from the context
func repositoryFromContext(ctx context.Context, path string) *git.Repository {
	value := ctx.Value(RepositoryContextKey)
	if value == nil {
		return nil
	}

	if repo, ok := value.(*git.Repository); ok {
		if repo.Path == path {
			return repo
		}
	}

	return nil
}

// RepositoryFromContextOrOpenPath attempts to get the repository from the context or just opens it
// Deprecated: Use RepositoryFromContextOrOpen instead
func RepositoryFromContextOrOpenPath(ctx context.Context, path string) (*git.Repository, io.Closer, error) {
	gitRepo := repositoryFromContext(ctx, path)
	if gitRepo != nil {
		return gitRepo, nopCloser(nil), nil
	}

	gitRepo, err := git.OpenRepository(ctx, path)
	return gitRepo, gitRepo, err
}
