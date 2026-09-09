// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoRunnerGroups(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	before, err := GetTasksVersionByScope(t.Context(), 0, 0)
	require.NoError(t, err)

	require.NoError(t, SetRepoRunnerGroups(t.Context(), 2, 1, []string{"gpu", "build"}))
	require.NoError(t, SetRepoRunnerGroups(t.Context(), 2, 1, []string{"gpu"}))
	names, err := GetRepoRunnerGroups(t.Context(), 1)
	require.NoError(t, err)
	assert.Equal(t, []string{"gpu"}, names)

	after, err := GetTasksVersionByScope(t.Context(), 0, 0)
	require.NoError(t, err)
	assert.Greater(t, after, before)

	require.NoError(t, db.Insert(t.Context(), &ActionRunner{Groups: []string{"build"}}))
	known, err := FindKnownRunnerGroupNames(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []string{"build", "gpu"}, known)
}
