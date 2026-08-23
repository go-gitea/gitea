// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanUserDelete(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 32})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 28})

	canDelete, err := CanUserDelete(t.Context(), repo, user)
	require.NoError(t, err)
	assert.False(t, canDelete)

	require.NoError(t, db.Insert(t.Context(),
		&repo_model.Collaboration{RepoID: repo.ID, UserID: user.ID, Mode: perm.AccessModeAdmin},
		&access_model.Access{RepoID: repo.ID, UserID: user.ID, Mode: perm.AccessModeAdmin},
	))
	canDelete, err = CanUserDelete(t.Context(), repo, user)
	require.NoError(t, err)
	assert.False(t, canDelete)

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 12})
	require.NoError(t, db.Insert(t.Context(), &organization.TeamRepo{OrgID: team.OrgID, TeamID: team.ID, RepoID: repo.ID}))
	canDelete, err = CanUserDelete(t.Context(), repo, user)
	require.NoError(t, err)
	assert.True(t, canDelete)

	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	canDelete, err = CanUserDelete(t.Context(), repo, owner)
	require.NoError(t, err)
	assert.True(t, canDelete)
	siteAdmin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	canDelete, err = CanUserDelete(t.Context(), repo, siteAdmin)
	require.NoError(t, err)
	assert.True(t, canDelete)
}
