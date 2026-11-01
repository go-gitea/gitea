// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateLanguageStatsPrimaryTie(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	require.NoError(t, repo_model.UpdateLanguageStats(t.Context(), repo, "1234567890", map[string]int64{"Go": 10, "C": 10, "Rust": 5}))

	stats, err := repo_model.GetLanguageStats(t.Context(), repo)
	require.NoError(t, err)
	require.Len(t, stats, 3)
	for _, stat := range stats {
		assert.Equal(t, stat.Language == "C", stat.IsPrimary, stat.Language)
	}
}
