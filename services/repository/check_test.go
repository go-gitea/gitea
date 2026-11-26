// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	repo_model "gitea.dev/models/repo"
	system_model "gitea.dev/models/system"
	"gitea.dev/models/unittest"
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

func TestMaintainRepositoryReportsFailure(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	repo := &repo_model.Repository{OwnerName: "owner", Name: t.Name()}
	repoPath := gitrepo.RepoLocalPath(repo)
	require.NoError(t, os.MkdirAll(repoPath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repoPath, "objects"), nil, 0o644))

	require.NoError(t, maintainRepository(t.Context(), repo))
	unittest.AssertExistsAndLoadBean(t, &system_model.Notice{Type: system_model.NoticeRepository}, unittest.Cond("description LIKE ?", "Failed to maintain "+repo.LogString()+":%"))
}
