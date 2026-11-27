// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateOwnerIDOfRepoLevelRunnerTokens(t *testing.T) {
	type ActionRunnerToken struct {
		ID       int64 `xorm:"pk autoincr"`
		OwnerID  int64
		RepoID   int64
		IsActive bool
	}

	x, deferrable := migrationtest.PrepareTestEnv(t, 0, new(ActionRunnerToken))
	defer deferrable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(
		&ActionRunnerToken{OwnerID: 2, RepoID: 1, IsActive: true},
		&ActionRunnerToken{OwnerID: 2, RepoID: 1},
		&ActionRunnerToken{OwnerID: 2, IsActive: true},
	)
	require.NoError(t, err)
	require.NoError(t, UpdateOwnerIDOfRepoLevelRunnerTokens(t.Context(), x))

	var tokens []ActionRunnerToken
	require.NoError(t, x.Asc("id").Find(&tokens))
	assert.Equal(t, []ActionRunnerToken{
		{ID: 1, OwnerID: 0, RepoID: 1, IsActive: true},
		{ID: 2, OwnerID: 2, RepoID: 1},
		{ID: 3, OwnerID: 2, IsActive: true},
	}, tokens)
}
