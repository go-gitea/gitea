// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"io"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/git"
)

type Repository interface {
	GetName() string
	GetOwnerName() string
}

type wikiRepository struct {
	Repository
}

func (w wikiRepository) GetName() string {
	return w.Repository.GetName() + ".wiki"
}

func wikiRepo(repo Repository) Repository {
	return wikiRepository{repo}
}

func repoRelativePath(repo Repository) string {
	return strings.ToLower(repo.GetOwnerName()) + "/" + strings.ToLower(repo.GetName()) + ".git"
}

// contextKey is a value for use with context.WithValue.
type contextKey struct {
	name string
}

// RepositoryContextKey is a context key. It is used with context.Value() to get the current Repository for the context
var RepositoryContextKey = &contextKey{"repository"}

// RepositoryFromContext attempts to get the repository from the context
func repositoryFromContext(ctx context.Context, repo Repository) *git.Repository {
	value := ctx.Value(RepositoryContextKey)
	if value == nil {
		return nil
	}

	if gitRepo, ok := value.(*git.Repository); ok && gitRepo != nil {
		relativePath := filepath.Join(strings.ToLower(repo.GetOwnerName()), strings.ToLower(repo.GetName()))
		if strings.HasSuffix(gitRepo.Path, relativePath) {
			return gitRepo
		}
	}

	return nil
}

type nopCloser func()

func (nopCloser) Close() error { return nil }

// RepositoryFromContextOrOpen attempts to get the repository from the context or just opens it
func RepositoryFromContextOrOpen(ctx context.Context, repo Repository) (*git.Repository, io.Closer, error) {
	gitRepo := repositoryFromContext(ctx, repo)
	if gitRepo != nil {
		return gitRepo, nopCloser(nil), nil
	}

	gitRepo, err := OpenRepository(ctx, repo)
	return gitRepo, gitRepo, err
}

// repositoryFromContextPath attempts to get the repository from the context
func repositoryFromContextPath(ctx context.Context, path string) *git.Repository {
	value := ctx.Value(RepositoryContextKey)
	if value == nil {
		return nil
	}

	if repo, ok := value.(*git.Repository); ok && repo != nil {
		if repo.Path == path {
			return repo
		}
	}

	return nil
}

// RepositoryFromContextOrOpenPath attempts to get the repository from the context or just opens it
// Deprecated: Use RepositoryFromContextOrOpen instead
func RepositoryFromContextOrOpenPath(ctx context.Context, path string) (*git.Repository, io.Closer, error) {
	gitRepo := repositoryFromContextPath(ctx, path)
	if gitRepo != nil {
		return gitRepo, nopCloser(nil), nil
	}

	gitRepo, err := git.OpenRepository(ctx, path)
	return gitRepo, gitRepo, err
}
