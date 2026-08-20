// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// When allPublic is set, the repository IDs handed to the issue indexer must not
// contain public repositories: the indexer matches those on its own, so
// enumerating them produces an IN list that grows with the size of the instance.
func TestSearchIssuesRepoIDsSkipsPublicRepos(t *testing.T) {
	unittest.PrepareTestEnv(t)

	cases := []struct {
		name string
		// user1 is a site administrator: SearchRepositoryCondition skips the
		// accessible repository condition for admins, so a regression enumerates
		// every row of the repository table.
		userID int64
	}{
		{name: "site admin", userID: 1},
		{name: "regular user", userID: 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: tc.userID})

			repoIDs, allPublic, err := SearchIssuesRepoIDs(t.Context(), SearchIssuesRepoIDsOptions{
				Doer:     doer,
				IsSigned: true,
			})
			require.NoError(t, err)
			assert.True(t, allPublic, "public repositories must be left to the indexer")
			// repo2 is private and owned by user2, and user1 is an administrator, so
			// both users reach it. Asserting it is still listed makes a filter that
			// narrows too far fail here rather than silently drop results.
			assert.Contains(t, repoIDs, int64(2))

			for _, repoID := range repoIDs {
				repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
				assert.True(t, repo.IsPrivate,
					"public repository %d is already covered by the indexer AllPublic filter", repoID)
			}
		})
	}
}

// An anonymous request reaches no private repository at all, so the whole filter
// collapses to the indexer's allPublic flag.
func TestSearchIssuesRepoIDsAnonymous(t *testing.T) {
	unittest.PrepareTestEnv(t)

	repoIDs, allPublic, err := SearchIssuesRepoIDs(t.Context(), SearchIssuesRepoIDsOptions{})
	require.NoError(t, err)
	assert.True(t, allPublic)
	// The placeholder keeps the indexer from falling back to "every repository".
	assert.Equal(t, []int64{0}, repoIDs)
}

// Filtering by owner turns the allPublic flag off, so the enumeration has to keep
// returning that owner's public repositories.
func TestSearchIssuesRepoIDsWithOwner(t *testing.T) {
	unittest.PrepareTestEnv(t)

	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	repoIDs, allPublic, err := SearchIssuesRepoIDs(t.Context(), SearchIssuesRepoIDsOptions{
		Doer:      doer,
		IsSigned:  true,
		OwnerName: "user2",
	})
	require.NoError(t, err)
	assert.False(t, allPublic)
	assert.Contains(t, repoIDs, int64(1), "public repository owned by user2")
	assert.Contains(t, repoIDs, int64(2), "private repository owned by user2")
}

func TestSearchIssuesRepoIDsTeamWithoutOwner(t *testing.T) {
	unittest.PrepareTestEnv(t)

	_, _, err := SearchIssuesRepoIDs(t.Context(), SearchIssuesRepoIDsOptions{
		TeamName: "team1",
	})
	assert.ErrorIs(t, err, util.ErrInvalidArgument)
}
