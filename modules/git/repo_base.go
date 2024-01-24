// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"io"
)

var isGogit bool

// contextKey is a value for use with context.WithValue.
type contextKey struct {
	name string
}

// RepositoryContextKey is a context key. It is used with context.Value() to get the current Repository for the context
var RepositoryContextKey = &contextKey{"repository"}

// RepositoryFromContext attempts to get the repository from the context
func RepositoryFromContext(ctx context.Context, path string) *Repository {
	value := ctx.Value(RepositoryContextKey)
	if value == nil {
		return nil
	}

	if repo, ok := value.(*Repository); ok && repo != nil {
		if repo.Path == path {
			return repo
		}
	}

	return nil
}

type nopCloser func()

func (nopCloser) Close() error { return nil }

// RepositoryFromContextOrOpen attempts to get the repository from the context or just opens it
func RepositoryFromContextOrOpen(ctx context.Context, path string) (*Repository, io.Closer, error) {
	gitRepo := RepositoryFromContext(ctx, path)
	if gitRepo != nil {
		return gitRepo, nopCloser(nil), nil
	}

	gitRepo, err := OpenRepository(ctx, path)
	return gitRepo, gitRepo, err
}
