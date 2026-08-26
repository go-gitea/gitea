// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanUserChangeTeamAccess(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 32})
	require.NoError(t, repo.LoadOwner(t.Context()))
	repoAdmin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 28})
	orgOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	siteAdmin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	canChange, err := CanDoerManageRepoCollaboratorTeam(t.Context(), nil, repo)
	require.NoError(t, err)
	assert.False(t, canChange)
	canChange, err = CanDoerManageRepoCollaboratorTeam(t.Context(), siteAdmin, repo)
	require.NoError(t, err)
	assert.True(t, canChange)
	canChange, err = CanDoerManageRepoCollaboratorTeam(t.Context(), orgOwner, repo)
	require.NoError(t, err)
	assert.True(t, canChange)

	repo.Owner.RepoAdminChangeTeamAccess = true
	canChange, err = CanDoerManageRepoCollaboratorTeam(t.Context(), repoAdmin, repo)
	require.NoError(t, err)
	assert.False(t, canChange)

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 12})
	require.NoError(t, db.Insert(t.Context(), &organization.TeamRepo{OrgID: team.OrgID, TeamID: team.ID, RepoID: repo.ID}))
	canChange, err = CanDoerManageRepoCollaboratorTeam(t.Context(), repoAdmin, repo)
	require.NoError(t, err)
	assert.True(t, canChange)

	repo.Owner.RepoAdminChangeTeamAccess = false
	canChange, err = CanDoerManageRepoCollaboratorTeam(t.Context(), repoAdmin, repo)
	require.NoError(t, err)
	assert.False(t, canChange)
}
