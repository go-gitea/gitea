// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRepoCatFileBatch(t *testing.T) {
	t.Run("MissingRepoAndClose", func(t *testing.T) {
		repo, err := OpenRepository(t.Context(), filepath.Join(testReposDir, "repo1_bare"))
		require.NoError(t, err)
		repo.Path = "/no-such" // when the repo is missing (it usually occurs during testing because the fixtures are synced frequently)
		_, _, err = repo.CatFileBatch(t.Context())
		require.Error(t, err)
		require.NoError(t, repo.Close()) // shouldn't panic
	})

	// TODO: test more methods and concurrency queries
}
