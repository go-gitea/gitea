// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git/gitrepo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaintainRepositoryRemovesStaleLocksInRepoAndWiki(t *testing.T) {
	repo := &repo_model.Repository{OwnerName: "owner", Name: t.Name()}
	mtime := time.Now().Add(-2 * time.Hour)
	var locks []string
	for _, gitRepo := range []gitrepo.RepositoryFacade{repo, repo.WikiStorageRepo()} {
		lock := filepath.Join(gitrepo.RepoLocalPath(gitRepo), "HEAD.lock")
		require.NoError(t, os.MkdirAll(filepath.Dir(lock), 0o755))
		require.NoError(t, os.WriteFile(lock, nil, 0o644))
		require.NoError(t, os.Chtimes(lock, mtime, mtime))
		locks = append(locks, lock)
	}

	require.NoError(t, maintainRepository(t.Context(), repo))
	for _, lock := range locks {
		assert.NoFileExists(t, lock)
	}
}
