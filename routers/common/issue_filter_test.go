// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"testing"

	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchIssuesRepoIDs(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

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
			doerID:    2,
			opts:      SearchIssuesRepoIDsOptions{PublicOnly: true},
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
}
