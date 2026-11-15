// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"slices"
	"testing"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchIssuesRepoIDs(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	publicOnlyUser2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	publicOnlyUser2.ExtDoerData = user_model.NewTokenExtDoerData(user_model.CredentialAccessToken, 1, "public-only,read:issue")

	// the indexer's is_public covers repo 1 (public under a public owner) but misses repo 38 (public
	// under a limited org) and repo 40 (public under a private org)
	cases := []struct {
		name      string
		doerID    int64
		opts      SearchIssuesRepoIDsOptions
		allPublic bool
		want      []int64
		wantErr   error
	}{
		{
			name:      "site admin", // admins skip the accessible repository condition entirely
			doerID:    1,
			allPublic: true,
			want:      []int64{2, 38, 40},
		},
		{
			name:      "regular user",
			doerID:    2,
			allPublic: true,
			want:      []int64{2, 38},
		},
		{
			name:      "private org member",
			doerID:    5,
			allPublic: true,
			want:      []int64{38, 40},
		},
		{
			name:      "anonymous",
			allPublic: true,
			want:      []int64{0}, // the placeholder keeps the indexer off "every repository"
		},
		{
			name:      "public-only token",
			opts:      SearchIssuesRepoIDsOptions{Doer: publicOnlyUser2},
			allPublic: true,
			want:      []int64{0},
		},
		{
			name:   "owner filter", // turns allPublic off, so public repos must still be enumerated
			doerID: 2,
			opts:   SearchIssuesRepoIDsOptions{OwnerName: "user2"},
			want:   []int64{1, 2},
		},
		{
			name:    "team without owner",
			opts:    SearchIssuesRepoIDsOptions{TeamName: "team1"},
			wantErr: util.ErrInvalidArgument,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := tc.opts
			if tc.doerID != 0 {
				opts.Doer = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: tc.doerID})
			}

			repoIDs, allPublic, err := SearchIssuesRepoIDs(t.Context(), opts)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.allPublic, allPublic)
			assert.Subset(t, repoIDs, tc.want)
			if allPublic {
				assert.NotContains(t, repoIDs, int64(1), "already matched by the indexer's is_public")
			}
		})
	}

	t.Run("team unit permissions on a private repository", func(t *testing.T) {
		t.Cleanup(func() { require.NoError(t, unittest.PrepareTestDatabase()) })
		_, err := db.Exec(t.Context(), "UPDATE team SET authorize = 0 WHERE id = 2")
		require.NoError(t, err)
		user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
		for _, tc := range []struct {
			unitType unit.Type
			isPull   optional.Option[bool]
			repoID   int64
		}{
			{unit.TypeIssues, optional.Some(false), 3},
			{unit.TypeIssues, optional.Some(true), 0},
			{unit.TypePullRequests, optional.Some(true), 3},
			{unit.TypeCode, optional.None[bool](), 0},
		} {
			_, err := db.Exec(t.Context(), "UPDATE team_unit SET access_mode = CASE WHEN type = ? THEN 1 ELSE 0 END WHERE team_id = 2", tc.unitType)
			require.NoError(t, err)
			repoIDs, _, err := SearchIssuesRepoIDs(t.Context(), SearchIssuesRepoIDsOptions{
				Doer: user4, OwnerName: "org3", TeamName: "team1", IsPull: tc.isPull,
			})
			require.NoError(t, err)
			assert.Equal(t, []int64{tc.repoID}, repoIDs, "unit %v, isPull %v", tc.unitType, tc.isPull)
		}
	})

	t.Run("disabled issues unit for anonymous, public-only and admin", func(t *testing.T) {
		t.Cleanup(func() { require.NoError(t, unittest.PrepareTestDatabase()) })
		_, err := db.GetEngine(t.Context()).In("repo_id", 1, 3).Delete(&repo_model.RepoUnit{Type: unit.TypeIssues})
		require.NoError(t, err)
		admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		for _, tc := range []struct {
			doer   *user_model.User
			owner  string
			repoID int64
		}{
			{nil, "user2", 1},
			{publicOnlyUser2, "user2", 1},
			{admin, "org3", 3},
		} {
			for _, isPull := range []bool{false, true} {
				repoIDs, _, err := SearchIssuesRepoIDs(t.Context(), SearchIssuesRepoIDsOptions{Doer: tc.doer, OwnerName: tc.owner, IsPull: optional.Some(isPull)})
				require.NoError(t, err)
				assert.Equal(t, isPull, slices.Contains(repoIDs, tc.repoID), "owner %s, isPull %v", tc.owner, isPull)
			}
		}
	})
}
