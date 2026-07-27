// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package uinotification

import (
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

// Watching a repository does not imply still being able to read it. A user who lost access
// must be dropped before any notification is written, for every source — this is what
// stops a private repo's commit or release titles leaking to them.
func TestFilterRecipientsByRepoAccess(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// repo 2 is private and owned by user 2; user 4 is unrelated to it
	privateRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	require.True(t, privateRepo.IsPrivate)

	allowed, err := filterRecipientsByRepoAccess(t.Context(), privateRepo, []int64{2, 4}, unit.TypeCode)
	require.NoError(t, err)
	assert.Equal(t, []int64{2}, allowed, "the owner keeps access, the outsider is dropped")

	allowed, err = filterRecipientsByRepoAccess(t.Context(), privateRepo, []int64{2, 4}, unit.TypeReleases)
	require.NoError(t, err)
	assert.Equal(t, []int64{2}, allowed, "the same rule applies to releases")
}

func TestFilterRecipientsByRepoAccessAllowsPublicRepo(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	publicRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	require.False(t, publicRepo.IsPrivate)

	allowed, err := filterRecipientsByRepoAccess(t.Context(), publicRepo, []int64{2, 4}, unit.TypeCode)
	require.NoError(t, err)
	assert.ElementsMatch(t, []int64{2, 4}, allowed)
}

func TestFilterRecipientsByRepoAccessWithNoRecipients(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	publicRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	allowed, err := filterRecipientsByRepoAccess(t.Context(), publicRepo, nil, unit.TypeCode)
	require.NoError(t, err)
	assert.Empty(t, allowed)
}
