// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestTeam_AddMember(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(teamID, userID int64) {
		team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID})
		assert.NoError(t, AddTeamMember(team, userID))
		unittest.AssertExistsAndLoadBean(t, &organization.TeamUser{UID: userID, TeamID: teamID})
		unittest.CheckConsistencyFor(t, &organization.Team{ID: teamID}, &user_model.User{ID: team.OrgID})
	}
	test(1, 2)
	test(1, 4)
	test(3, 2)
}

func TestTeam_RemoveMember(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(teamID, userID int64) {
		team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID})
		assert.NoError(t, RemoveTeamMember(team, userID))
		unittest.AssertNotExistsBean(t, &organization.TeamUser{UID: userID, TeamID: teamID})
		unittest.CheckConsistencyFor(t, &organization.Team{ID: teamID})
	}
	testSuccess(1, 4)
	testSuccess(2, 2)
	testSuccess(3, 2)
	testSuccess(3, unittest.NonexistentID)

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 1})
	err := RemoveTeamMember(team, 2)
	assert.True(t, organization.IsErrLastOrgOwner(err))
}

func TestTeam_HasRepository(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(teamID, repoID int64, expected bool) {
		team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID})
		assert.Equal(t, expected, HasRepository(team, repoID))
	}
	test(1, 1, false)
	test(1, 3, true)
	test(1, 5, true)
	test(1, unittest.NonexistentID, false)

	test(2, 3, true)
	test(2, 5, false)
}

func TestTeam_RemoveRepository(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(teamID, repoID int64) {
		team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID})
		assert.NoError(t, RemoveRepository(team, repoID))
		unittest.AssertNotExistsBean(t, &organization.TeamRepo{TeamID: teamID, RepoID: repoID})
		unittest.CheckConsistencyFor(t, &organization.Team{ID: teamID}, &repo_model.Repository{ID: repoID})
	}
	testSuccess(2, 3)
	testSuccess(2, 5)
	testSuccess(1, unittest.NonexistentID)
}

func TestIsUsableTeamName(t *testing.T) {
	assert.NoError(t, organization.IsUsableTeamName("usable"))
	assert.True(t, db.IsErrNameReserved(organization.IsUsableTeamName("new")))
}

func TestNewTeam(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const teamName = "newTeamName"
	team := &organization.Team{Name: teamName, OrgID: 3}
	assert.NoError(t, NewTeam(team))
	unittest.AssertExistsAndLoadBean(t, &organization.Team{Name: teamName})
	unittest.CheckConsistencyFor(t, &organization.Team{}, &user_model.User{ID: team.OrgID})
}

func TestUpdateTeam(t *testing.T) {
	// successful update
	assert.NoError(t, unittest.PrepareTestDatabase())

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	team.LowerName = "newname"
	team.Name = "newName"
	team.Description = strings.Repeat("A long description!", 100)
	team.AccessMode = perm.AccessModeAdmin
	assert.NoError(t, UpdateTeam(team, true, false))

	team = unittest.AssertExistsAndLoadBean(t, &organization.Team{Name: "newName"})
	assert.True(t, strings.HasPrefix(team.Description, "A long description!"))

	access := unittest.AssertExistsAndLoadBean(t, &access_model.Access{UserID: 4, RepoID: 3})
	assert.EqualValues(t, perm.AccessModeAdmin, access.Mode)

	unittest.CheckConsistencyFor(t, &organization.Team{ID: team.ID})
}

func TestUpdateTeam2(t *testing.T) {
	// update to already-existing team
	assert.NoError(t, unittest.PrepareTestDatabase())

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	team.LowerName = "owners"
	team.Name = "Owners"
	team.Description = strings.Repeat("A long description!", 100)
	err := UpdateTeam(team, true, false)
	assert.True(t, organization.IsErrTeamAlreadyExist(err))

	unittest.CheckConsistencyFor(t, &organization.Team{ID: team.ID})
}

func TestDeleteTeam(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	assert.NoError(t, DeleteTeam(team))
	unittest.AssertNotExistsBean(t, &organization.Team{ID: team.ID})
	unittest.AssertNotExistsBean(t, &organization.TeamRepo{TeamID: team.ID})
	unittest.AssertNotExistsBean(t, &organization.TeamUser{TeamID: team.ID})

	// check that team members don't have "leftover" access to repos
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	accessMode, err := access_model.AccessLevel(user, repo)
	assert.NoError(t, err)
	assert.True(t, accessMode < perm.AccessModeWrite)
}

func TestAddTeamMember(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(teamID, userID int64) {
		team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID})
		assert.NoError(t, AddTeamMember(team, userID))
		unittest.AssertExistsAndLoadBean(t, &organization.TeamUser{UID: userID, TeamID: teamID})
		unittest.CheckConsistencyFor(t, &organization.Team{ID: teamID}, &user_model.User{ID: team.OrgID})
	}
	test(1, 2)
	test(1, 4)
	test(3, 2)
}

func TestRemoveTeamMember(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(teamID, userID int64) {
		team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID})
		assert.NoError(t, RemoveTeamMember(team, userID))
		unittest.AssertNotExistsBean(t, &organization.TeamUser{UID: userID, TeamID: teamID})
		unittest.CheckConsistencyFor(t, &organization.Team{ID: teamID})
	}
	testSuccess(1, 4)
	testSuccess(2, 2)
	testSuccess(3, 2)
	testSuccess(3, unittest.NonexistentID)

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 1})
	err := RemoveTeamMember(team, 2)
	assert.True(t, organization.IsErrLastOrgOwner(err))
}

func TestRepository_RecalculateAccesses3(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	team5 := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 5})
	user29 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 29})

	has, err := db.GetEngine(db.DefaultContext).Get(&access_model.Access{UserID: 29, RepoID: 23})
	assert.NoError(t, err)
	assert.False(t, has)

	// adding user29 to team5 should add an explicit access row for repo 23
	// even though repo 23 is public
	assert.NoError(t, AddTeamMember(team5, user29.ID))

	has, err = db.GetEngine(db.DefaultContext).Get(&access_model.Access{UserID: 29, RepoID: 23})
	assert.NoError(t, err)
	assert.True(t, has)
}
