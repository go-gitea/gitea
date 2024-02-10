// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"fmt"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/git"
)

type Repository interface {
	GetName() string
	GetOwnerName() string
}

var _ Repository = (*SimpleRepository)(nil)

type SimpleRepository struct {
	OwnerName string
	Name      string
}

func (r *SimpleRepository) GetName() string {
	return r.Name
}

func (r *SimpleRepository) GetOwnerName() string {
	return r.OwnerName
}

func repoRelativePath(repo Repository) string {
	return strings.ToLower(repo.GetOwnerName()) + "/" + strings.ToLower(repo.GetName()) + ".git"
}

func wikiRelativePath(repo Repository) string {
	return strings.ToLower(repo.GetOwnerName()) + "/" + strings.ToLower(repo.GetName()) + ".wiki.git"
}

// contextKey is a value for use with context.WithValue.
type contextKey struct {
	name string
}

// RepositoryContextKey is a context key. It is used with context.Value() to get the current Repository for the context
var RepositoryContextKey = &contextKey{"repository"}

// RepositoryFromContext attempts to get the repository from the context
func repositoryFromContext(ctx context.Context, repo Repository) GitRepository {
	value := ctx.Value(RepositoryContextKey)
	if value == nil {
		return nil
	}

	if gitRepo, ok := value.(GitRepository); ok && gitRepo != nil {
		if gitRepo.GetRelativePath() == repoRelativePath(repo) {
			return gitRepo
		}
	}

	return nil
}

type nopCloser func()

func (nopCloser) Close() error { return nil }

// RepositoryFromContextOrOpen attempts to get the repository from the context or just opens it
func RepositoryFromContextOrOpen(ctx context.Context, repo Repository) (GitRepository, io.Closer, error) {
	gitRepo := repositoryFromContext(ctx, repo)
	if gitRepo != nil {
		return gitRepo, nopCloser(nil), nil
	}

	gitRepo, err := OpenRepository(ctx, repo)
	return gitRepo, gitRepo, err
}

// repositoryFromContextPath attempts to get the repository from the context
func repositoryFromContextPath(ctx context.Context, path string) GitRepository {
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
func RepositoryFromContextOrOpenPath(ctx context.Context, path string) (GitRepository, io.Closer, error) {
	gitRepo := repositoryFromContextPath(ctx, path)
	if gitRepo != nil {
		return gitRepo, nopCloser(nil), nil
	}

	gitRepo, err := git.OpenRepository(ctx, path)
	return gitRepo, gitRepo, err
}

func IsRepositoryExist(ctx context.Context, repo Repository) (bool, error) {
	return curService.IsRepositoryExist(ctx, repoRelativePath(repo))
}

func RenameRepository(ctx context.Context, repo Repository, newName string) error {
	newRepoPath := strings.ToLower(repo.GetOwnerName()) + "/" + strings.ToLower(newName) + ".git"
	if err := curService.RenameDir(repoRelativePath(repo), newRepoPath); err != nil {
		return fmt.Errorf("rename repository directory: %w", err)
	}
	return nil
}

func RenameWikiRepository(ctx context.Context, repo Repository, newName string) error {
	newWikiRepoPath := strings.ToLower(repo.GetOwnerName()) + "/" + strings.ToLower(newName) + ".wiki.git"
	if err := curService.RenameDir(wikiRelativePath(repo), newWikiRepoPath); err != nil {
		return fmt.Errorf("rename repository wiki directory: %w", err)
	}
	return nil
}

func DeleteRepository(ctx context.Context, repo Repository) error {
	return curService.RemoveDir(repoRelativePath(repo))
}

func ForkRepository(ctx context.Context, baseRepo, targetRepo Repository, singleBranch string) error {
	return curService.ForkRepository(ctx, repoRelativePath(baseRepo), repoRelativePath(targetRepo), singleBranch)
}
