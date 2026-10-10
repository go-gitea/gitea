// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package goproxy

import (
	"context"
	"net/url"
	"path/filepath"

	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/setting"
)

// NewDirectRepository creates a temporary bare clone for a module path fetched
// directly from its VCS. The returned cleanup function removes the clone.
func NewDirectRepository(ctx context.Context, modulePath string) (*Repository, func(), error) {
	directURL, err := url.Parse("https://" + modulePath)
	if err != nil || directURL.Scheme != "https" {
		return nil, nil, ErrInvalidVersion
	}

	tmpDir, cleanup, err := setting.AppDataTempDir("goproxy-direct").MkdirTempRandom("repo")
	if err != nil {
		return nil, nil, err
	}

	clonePath := filepath.Join(tmpDir, "repo.git")
	if err := git.Clone(ctx, directURL.String(), clonePath, git.CloneRepoOptions{Bare: true}); err != nil {
		cleanup()
		return nil, nil, err
	}

	return &Repository{
		RepoFacade: gitrepo.RepositoryUnmanaged(clonePath),
		ModulePath: modulePath,
	}, cleanup, nil
}
