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

func TestSearchIssuesRepoIDsSkipsPublicRepos(t *testing.T) {
	unittest.PrepareTestEnv(t)

	cases := []struct {
		name   string
		userID int64
	}{
		{name: "site admin", userID: 1}, // admins skip the accessible repository condition entirely
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
			assert.Contains(t, repoIDs, int64(2)) // a private repo both users reach, so narrowing too far also fails

			for _, repoID := range repoIDs {
				repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
				assert.True(t, repo.IsPrivate,
					"public repository %d is already covered by the indexer AllPublic filter", repoID)
			}
		})
	}
}

func TestSearchIssuesRepoIDsAnonymous(t *testing.T) {
	unittest.PrepareTestEnv(t)

	repoIDs, allPublic, err := SearchIssuesRepoIDs(t.Context(), SearchIssuesRepoIDsOptions{})
	require.NoError(t, err)
	assert.True(t, allPublic)
	assert.Equal(t, []int64{0}, repoIDs) // the placeholder keeps the indexer off "every repository"
}

// Filtering by owner turns allPublic off, so public repos must still be enumerated.
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
