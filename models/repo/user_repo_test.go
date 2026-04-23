// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	perm_model "code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepo(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	t.Run("GetIssuePostersWithSearch", testUserRepoGetIssuePostersWithSearch)
	t.Run("Assignees", testUserRepoAssignees)
	t.Run("AssigneesNoTeamUnit", testRepoAssigneesNoTeamUnit)
}

func testUserRepoAssignees(t *testing.T) {
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	users, err := repo_model.GetRepoAssignees(t.Context(), repo2)
	assert.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, int64(2), users[0].ID)

	repo21 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 21})
	users, err = repo_model.GetRepoAssignees(t.Context(), repo21)
	assert.NoError(t, err)
	if assert.Len(t, users, 4) {
		assert.ElementsMatch(t, []int64{10, 15, 16, 18}, []int64{users[0].ID, users[1].ID, users[2].ID, users[3].ID})
	}

	// do not return deactivated users
	assert.NoError(t, user_model.UpdateUserCols(t.Context(), &user_model.User{ID: 15, IsActive: false}, "is_active"))
	users, err = repo_model.GetRepoAssignees(t.Context(), repo21)
	assert.NoError(t, err)
	if assert.Len(t, users, 3) {
		assert.NotContains(t, []int64{users[0].ID, users[1].ID, users[2].ID}, 15)
	}
}

func testRepoAssigneesNoTeamUnit(t *testing.T) {
	ctx := t.Context()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 32})
	require.NoError(t, repo.LoadOwner(ctx))
	require.True(t, repo.Owner.IsOrganization())

	require.NoError(t, db.TruncateBeans(ctx, &organization.Team{}, &organization.TeamUser{}, &organization.TeamRepo{}, &organization.TeamUnit{}, &access_model.Access{}))

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	team := &organization.Team{OrgID: repo.OwnerID, LowerName: "admin-team", AccessMode: perm_model.AccessModeAdmin}
	require.NoError(t, db.Insert(ctx, team))
	require.NoError(t, db.Insert(ctx, &organization.TeamUser{OrgID: repo.OwnerID, TeamID: team.ID, UID: user.ID}))
	require.NoError(t, db.Insert(ctx, &organization.TeamRepo{OrgID: repo.OwnerID, TeamID: team.ID, RepoID: repo.ID}))
	require.NoError(t, db.Insert(ctx, &organization.TeamUnit{OrgID: repo.OwnerID, TeamID: team.ID, Type: unit.TypePullRequests, AccessMode: perm_model.AccessModeNone}))

	users, err := repo_model.GetRepoAssignees(ctx, repo)
	require.NoError(t, err)
	require.Len(t, users, 1)
	assert.ElementsMatch(t, []int64{4}, []int64{users[0].ID})
}

func testUserRepoGetIssuePostersWithSearch(t *testing.T) {
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})

	users, err := repo_model.GetIssuePostersWithSearch(t.Context(), repo2, false, "USER")
	require.NoError(t, err)
	require.Len(t, users, 1)
	assert.Equal(t, "user2", users[0].Name)

	users, err = repo_model.GetIssuePostersWithSearch(t.Context(), repo2, false, "TW%O")
	require.NoError(t, err)
	require.Len(t, users, 1)
	assert.Equal(t, "user2", users[0].Name)
}
