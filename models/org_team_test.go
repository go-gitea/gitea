// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

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

	test := func(team *organization.Team, user *user_model.User) {
		assert.NoError(t, AddTeamMember(db.DefaultContext, team, user))
		unittest.AssertExistsAndLoadBean(t, &organization.TeamUser{UID: user.ID, TeamID: team.ID})
		unittest.CheckConsistencyFor(t, &organization.Team{ID: team.ID}, &user_model.User{ID: team.OrgID})
	}

	team1 := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 1})
	team3 := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 3})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	test(team1, user2)
	test(team1, user4)
	test(team3, user2)
}

func TestTeam_RemoveMember(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(team *organization.Team, user *user_model.User) {
		assert.NoError(t, RemoveTeamMember(db.DefaultContext, team, user))
		unittest.AssertNotExistsBean(t, &organization.TeamUser{UID: user.ID, TeamID: team.ID})
		unittest.CheckConsistencyFor(t, &organization.Team{ID: team.ID})
	}

	team1 := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 1})
	team2 := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	team3 := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 3})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	testSuccess(team1, user4)
	testSuccess(team2, user2)
	testSuccess(team3, user2)

	err := RemoveTeamMember(db.DefaultContext, team1, user2)
	assert.True(t, organization.IsErrLastOrgOwner(err))
}

func TestIsUsableTeamName(t *testing.T) {
	assert.NoError(t, organization.IsUsableTeamName("usable"))
	assert.True(t, db.IsErrNameReserved(organization.IsUsableTeamName("new")))
}

func TestNewTeam(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const teamName = "newTeamName"
	team := &organization.Team{Name: teamName, OrgID: 3}
	assert.NoError(t, NewTeam(db.DefaultContext, team))
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
	assert.NoError(t, UpdateTeam(db.DefaultContext, team, true, false))

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
	err := UpdateTeam(db.DefaultContext, team, true, false)
	assert.True(t, organization.IsErrTeamAlreadyExist(err))

	unittest.CheckConsistencyFor(t, &organization.Team{ID: team.ID})
}

func TestDeleteTeam(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	assert.NoError(t, DeleteTeam(db.DefaultContext, team))
	unittest.AssertNotExistsBean(t, &organization.Team{ID: team.ID})
	unittest.AssertNotExistsBean(t, &organization.TeamRepo{TeamID: team.ID})
	unittest.AssertNotExistsBean(t, &organization.TeamUser{TeamID: team.ID})

	// check that team members don't have "leftover" access to repos
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	accessMode, err := access_model.AccessLevel(db.DefaultContext, user, repo)
	assert.NoError(t, err)
	assert.True(t, accessMode < perm.AccessModeWrite)
}

func TestAddTeamMember(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(team *organization.Team, user *user_model.User) {
		assert.NoError(t, AddTeamMember(db.DefaultContext, team, user))
		unittest.AssertExistsAndLoadBean(t, &organization.TeamUser{UID: user.ID, TeamID: team.ID})
		unittest.CheckConsistencyFor(t, &organization.Team{ID: team.ID}, &user_model.User{ID: team.OrgID})
	}

	team1 := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 1})
	team3 := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 3})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	test(team1, user2)
	test(team1, user4)
	test(team3, user2)
}

func TestRemoveTeamMember(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(team *organization.Team, user *user_model.User) {
		assert.NoError(t, RemoveTeamMember(db.DefaultContext, team, user))
		unittest.AssertNotExistsBean(t, &organization.TeamUser{UID: user.ID, TeamID: team.ID})
		unittest.CheckConsistencyFor(t, &organization.Team{ID: team.ID})
	}

	team1 := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 1})
	team2 := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	team3 := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 3})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	testSuccess(team1, user4)
	testSuccess(team2, user2)
	testSuccess(team3, user2)

	err := RemoveTeamMember(db.DefaultContext, team1, user2)
	assert.True(t, organization.IsErrLastOrgOwner(err))
}

func TestRepository_RecalculateAccesses3(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	team5 := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 5})
	user29 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 29})

	has, err := db.GetEngine(db.DefaultContext).Get(&access_model.Access{UserID: user29.ID, RepoID: 23})
	assert.NoError(t, err)
	assert.False(t, has)

	// adding user29 to team5 should add an explicit access row for repo 23
	// even though repo 23 is public
	assert.NoError(t, AddTeamMember(db.DefaultContext, team5, user29))

	has, err = db.GetEngine(db.DefaultContext).Get(&access_model.Access{UserID: user29.ID, RepoID: 23})
	assert.NoError(t, err)
	assert.True(t, has)
}
