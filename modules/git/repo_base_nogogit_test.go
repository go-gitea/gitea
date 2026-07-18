// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRepoCatFileBatch(t *testing.T) {
	t.Run("MissingRepoAndClose", func(t *testing.T) {
		testDir := filepath.Join(t.TempDir(), "testdir")
		_ = os.Mkdir(testDir, 0o755)
		repo, err := OpenRepositoryLocal(testDir)
		require.NoError(t, err)
		// when the repo is missing (it usually occurs during testing because the fixtures are synced frequently)
		err = os.Remove(testDir)
		require.NoError(t, err)
		_, _, err = repo.CatFileBatch(t.Context())
		require.Error(t, err)
		require.NoError(t, repo.Close()) // shouldn't panic
	})

	// TODO: test more methods and concurrency queries
}
