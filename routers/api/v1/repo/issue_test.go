// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The repository IDs handed to the indexer must never contain public repositories
// when allPublic is set: the indexer already matches those on its own, so
// enumerating them produces an IN list that grows with the size of the instance.
func TestBuildSearchIssuesRepoIDsSkipsPublicRepos(t *testing.T) {
	unittest.PrepareTestEnv(t)

	cases := []struct {
		name   string
		userID int64
		// mustContain is a private repository the user can reach, so that narrowing
		// the query too far fails here instead of silently dropping results.
		mustContain int64
	}{
		// user1 is a site administrator: SearchRepositoryCondition skips the
		// accessible repository condition for admins, so a regression enumerates
		// every row of the repository table.
		{name: "site admin", userID: 1, mustContain: 2},
		{name: "regular user", userID: 2, mustContain: 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := contexttest.MockAPIContext(t, "/api/v1/repos/issues/search")
			contexttest.LoadUser(t, ctx, tc.userID)

			repoIDs, allPublic, err := buildSearchIssuesRepoIDs(ctx)
			require.NoError(t, err)
			assert.True(t, allPublic, "public repositories must be left to the indexer")
			require.NotEmpty(t, repoIDs)
			assert.Contains(t, repoIDs, tc.mustContain)

			for _, repoID := range repoIDs {
				repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
				assert.True(t, repo.IsPrivate,
					"public repository %d is already covered by the indexer AllPublic filter", repoID)
			}
		})
	}
}

// Filtering by owner turns the AllPublic flag off, so the enumeration has to keep
// returning that owner's public repositories.
func TestBuildSearchIssuesRepoIDsWithOwner(t *testing.T) {
	unittest.PrepareTestEnv(t)

	ctx, _ := contexttest.MockAPIContext(t, "/api/v1/repos/issues/search?owner=user2")
	contexttest.LoadUser(t, ctx, 2)

	repoIDs, allPublic, err := buildSearchIssuesRepoIDs(ctx)
	require.NoError(t, err)
	assert.False(t, allPublic)
	assert.Contains(t, repoIDs, int64(1), "public repository owned by user2")
	assert.Contains(t, repoIDs, int64(2), "private repository owned by user2")
}
