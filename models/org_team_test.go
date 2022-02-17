// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestTeam_IsOwnerTeam(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	team := unittest.AssertExistsAndLoadBean(t, &Team{ID: 1}).(*Team)
	assert.True(t, team.IsOwnerTeam())

	team = unittest.AssertExistsAndLoadBean(t, &Team{ID: 2}).(*Team)
	assert.False(t, team.IsOwnerTeam())
}

func TestTeam_IsMember(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	team := unittest.AssertExistsAndLoadBean(t, &Team{ID: 1}).(*Team)
	assert.True(t, team.IsMember(2))
	assert.False(t, team.IsMember(4))
	assert.False(t, team.IsMember(unittest.NonexistentID))

	team = unittest.AssertExistsAndLoadBean(t, &Team{ID: 2}).(*Team)
	assert.True(t, team.IsMember(2))
	assert.True(t, team.IsMember(4))
	assert.False(t, team.IsMember(unittest.NonexistentID))
}

func TestTeam_GetRepositories(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(teamID int64) {
		team := unittest.AssertExistsAndLoadBean(t, &Team{ID: teamID}).(*Team)
		assert.NoError(t, team.GetRepositories(&SearchOrgTeamOptions{}))
		assert.Len(t, team.Repos, team.NumRepos)
		for _, repo := range team.Repos {
			unittest.AssertExistsAndLoadBean(t, &TeamRepo{TeamID: teamID, RepoID: repo.ID})
		}
	}
	test(1)
	test(3)
}

func TestTeam_GetMembers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(teamID int64) {
		team := unittest.AssertExistsAndLoadBean(t, &Team{ID: teamID}).(*Team)
		assert.NoError(t, team.GetMembers(&SearchMembersOptions{}))
		assert.Len(t, team.Members, team.NumMembers)
		for _, member := range team.Members {
			unittest.AssertExistsAndLoadBean(t, &TeamUser{UID: member.ID, TeamID: teamID})
		}
	}
	test(1)
	test(3)
}

func TestTeam_AddMember(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(teamID, userID int64) {
		team := unittest.AssertExistsAndLoadBean(t, &Team{ID: teamID}).(*Team)
		assert.NoError(t, team.AddMember(userID))
		unittest.AssertExistsAndLoadBean(t, &TeamUser{UID: userID, TeamID: teamID})
		unittest.CheckConsistencyFor(t, &Team{ID: teamID}, &user_model.User{ID: team.OrgID})
	}
	test(1, 2)
	test(1, 4)
	test(3, 2)
}

func TestTeam_RemoveMember(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(teamID, userID int64) {
		team := unittest.AssertExistsAndLoadBean(t, &Team{ID: teamID}).(*Team)
		assert.NoError(t, team.RemoveMember(userID))
		unittest.AssertNotExistsBean(t, &TeamUser{UID: userID, TeamID: teamID})
		unittest.CheckConsistencyFor(t, &Team{ID: teamID})
	}
	testSuccess(1, 4)
	testSuccess(2, 2)
	testSuccess(3, 2)
	testSuccess(3, unittest.NonexistentID)

	team := unittest.AssertExistsAndLoadBean(t, &Team{ID: 1}).(*Team)
	err := team.RemoveMember(2)
	assert.True(t, IsErrLastOrgOwner(err))
}

func TestTeam_HasRepository(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(teamID, repoID int64, expected bool) {
		team := unittest.AssertExistsAndLoadBean(t, &Team{ID: teamID}).(*Team)
		assert.Equal(t, expected, team.HasRepository(repoID))
	}
	test(1, 1, false)
	test(1, 3, true)
	test(1, 5, true)
	test(1, unittest.NonexistentID, false)

	test(2, 3, true)
	test(2, 5, false)
}

func TestTeam_AddRepository(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(teamID, repoID int64) {
		team := unittest.AssertExistsAndLoadBean(t, &Team{ID: teamID}).(*Team)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID}).(*repo_model.Repository)
		assert.NoError(t, team.AddRepository(repo))
		unittest.AssertExistsAndLoadBean(t, &TeamRepo{TeamID: teamID, RepoID: repoID})
		unittest.CheckConsistencyFor(t, &Team{ID: teamID}, &repo_model.Repository{ID: repoID})
	}
	testSuccess(2, 3)
	testSuccess(2, 5)

	team := unittest.AssertExistsAndLoadBean(t, &Team{ID: 1}).(*Team)
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)
	assert.Error(t, team.AddRepository(repo))
	unittest.CheckConsistencyFor(t, &Team{ID: 1}, &repo_model.Repository{ID: 1})
}

func TestTeam_RemoveRepository(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(teamID, repoID int64) {
		team := unittest.AssertExistsAndLoadBean(t, &Team{ID: teamID}).(*Team)
		assert.NoError(t, team.RemoveRepository(repoID))
		unittest.AssertNotExistsBean(t, &TeamRepo{TeamID: teamID, RepoID: repoID})
		unittest.CheckConsistencyFor(t, &Team{ID: teamID}, &repo_model.Repository{ID: repoID})
	}
	testSuccess(2, 3)
	testSuccess(2, 5)
	testSuccess(1, unittest.NonexistentID)
}

func TestIsUsableTeamName(t *testing.T) {
	assert.NoError(t, IsUsableTeamName("usable"))
	assert.True(t, db.IsErrNameReserved(IsUsableTeamName("new")))
}

func TestNewTeam(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const teamName = "newTeamName"
	team := &Team{Name: teamName, OrgID: 3}
	assert.NoError(t, NewTeam(team))
	unittest.AssertExistsAndLoadBean(t, &Team{Name: teamName})
	unittest.CheckConsistencyFor(t, &Team{}, &user_model.User{ID: team.OrgID})
}

func TestGetTeam(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(orgID int64, name string) {
		team, err := GetTeam(orgID, name)
		assert.NoError(t, err)
		assert.EqualValues(t, orgID, team.OrgID)
		assert.Equal(t, name, team.Name)
	}
	testSuccess(3, "Owners")
	testSuccess(3, "team1")

	_, err := GetTeam(3, "nonexistent")
	assert.Error(t, err)
	_, err = GetTeam(unittest.NonexistentID, "Owners")
	assert.Error(t, err)
}

func TestGetTeamByID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(teamID int64) {
		team, err := GetTeamByID(teamID)
		assert.NoError(t, err)
		assert.EqualValues(t, teamID, team.ID)
	}
	testSuccess(1)
	testSuccess(2)
	testSuccess(3)
	testSuccess(4)

	_, err := GetTeamByID(unittest.NonexistentID)
	assert.Error(t, err)
}

func TestUpdateTeam(t *testing.T) {
	// successful update
	assert.NoError(t, unittest.PrepareTestDatabase())

	team := unittest.AssertExistsAndLoadBean(t, &Team{ID: 2}).(*Team)
	team.LowerName = "newname"
	team.Name = "newName"
	team.Description = strings.Repeat("A long description!", 100)
	team.AccessMode = perm.AccessModeAdmin
	assert.NoError(t, UpdateTeam(team, true, false))

	team = unittest.AssertExistsAndLoadBean(t, &Team{Name: "newName"}).(*Team)
	assert.True(t, strings.HasPrefix(team.Description, "A long description!"))

	access := unittest.AssertExistsAndLoadBean(t, &Access{UserID: 4, RepoID: 3}).(*Access)
	assert.EqualValues(t, perm.AccessModeAdmin, access.Mode)

	unittest.CheckConsistencyFor(t, &Team{ID: team.ID})
}

func TestUpdateTeam2(t *testing.T) {
	// update to already-existing team
	assert.NoError(t, unittest.PrepareTestDatabase())

	team := unittest.AssertExistsAndLoadBean(t, &Team{ID: 2}).(*Team)
	team.LowerName = "owners"
	team.Name = "Owners"
	team.Description = strings.Repeat("A long description!", 100)
	err := UpdateTeam(team, true, false)
	assert.True(t, IsErrTeamAlreadyExist(err))

	unittest.CheckConsistencyFor(t, &Team{ID: team.ID})
}

func TestDeleteTeam(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	team := unittest.AssertExistsAndLoadBean(t, &Team{ID: 2}).(*Team)
	assert.NoError(t, DeleteTeam(team))
	unittest.AssertNotExistsBean(t, &Team{ID: team.ID})
	unittest.AssertNotExistsBean(t, &TeamRepo{TeamID: team.ID})
	unittest.AssertNotExistsBean(t, &TeamUser{TeamID: team.ID})

	// check that team members don't have "leftover" access to repos
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4}).(*user_model.User)
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3}).(*repo_model.Repository)
	accessMode, err := AccessLevel(user, repo)
	assert.NoError(t, err)
	assert.True(t, accessMode < perm.AccessModeWrite)
}

func TestIsTeamMember(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	test := func(orgID, teamID, userID int64, expected bool) {
		isMember, err := IsTeamMember(orgID, teamID, userID)
		assert.NoError(t, err)
		assert.Equal(t, expected, isMember)
	}

	test(3, 1, 2, true)
	test(3, 1, 4, false)
	test(3, 1, unittest.NonexistentID, false)

	test(3, 2, 2, true)
	test(3, 2, 4, true)

	test(3, unittest.NonexistentID, unittest.NonexistentID, false)
	test(unittest.NonexistentID, unittest.NonexistentID, unittest.NonexistentID, false)
}

func TestGetTeamMembers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(teamID int64) {
		team := unittest.AssertExistsAndLoadBean(t, &Team{ID: teamID}).(*Team)
		members, err := GetTeamMembers(teamID)
		assert.NoError(t, err)
		assert.Len(t, members, team.NumMembers)
		for _, member := range members {
			unittest.AssertExistsAndLoadBean(t, &TeamUser{UID: member.ID, TeamID: teamID})
		}
	}
	test(1)
	test(3)
}

func TestGetUserTeams(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	test := func(userID int64) {
		teams, _, err := GetUserTeams(&GetUserTeamOptions{UserID: userID})
		assert.NoError(t, err)
		for _, team := range teams {
			unittest.AssertExistsAndLoadBean(t, &TeamUser{TeamID: team.ID, UID: userID})
		}
	}
	test(2)
	test(5)
	test(unittest.NonexistentID)
}

func TestGetUserOrgTeams(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	test := func(orgID, userID int64) {
		teams, err := GetUserOrgTeams(orgID, userID)
		assert.NoError(t, err)
		for _, team := range teams {
			assert.EqualValues(t, orgID, team.OrgID)
			unittest.AssertExistsAndLoadBean(t, &TeamUser{TeamID: team.ID, UID: userID})
		}
	}
	test(3, 2)
	test(3, 4)
	test(3, unittest.NonexistentID)
}

func TestAddTeamMember(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(teamID, userID int64) {
		team := unittest.AssertExistsAndLoadBean(t, &Team{ID: teamID}).(*Team)
		assert.NoError(t, AddTeamMember(team, userID))
		unittest.AssertExistsAndLoadBean(t, &TeamUser{UID: userID, TeamID: teamID})
		unittest.CheckConsistencyFor(t, &Team{ID: teamID}, &user_model.User{ID: team.OrgID})
	}
	test(1, 2)
	test(1, 4)
	test(3, 2)
}

func TestRemoveTeamMember(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(teamID, userID int64) {
		team := unittest.AssertExistsAndLoadBean(t, &Team{ID: teamID}).(*Team)
		assert.NoError(t, RemoveTeamMember(team, userID))
		unittest.AssertNotExistsBean(t, &TeamUser{UID: userID, TeamID: teamID})
		unittest.CheckConsistencyFor(t, &Team{ID: teamID})
	}
	testSuccess(1, 4)
	testSuccess(2, 2)
	testSuccess(3, 2)
	testSuccess(3, unittest.NonexistentID)

	team := unittest.AssertExistsAndLoadBean(t, &Team{ID: 1}).(*Team)
	err := RemoveTeamMember(team, 2)
	assert.True(t, IsErrLastOrgOwner(err))
}

func TestHasTeamRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(teamID, repoID int64, expected bool) {
		team := unittest.AssertExistsAndLoadBean(t, &Team{ID: teamID}).(*Team)
		assert.Equal(t, expected, HasTeamRepo(team.OrgID, teamID, repoID))
	}
	test(1, 1, false)
	test(1, 3, true)
	test(1, 5, true)
	test(1, unittest.NonexistentID, false)

	test(2, 3, true)
	test(2, 5, false)
}

func TestUsersInTeamsCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(teamIDs, userIDs []int64, expected int64) {
		count, err := UsersInTeamsCount(teamIDs, userIDs)
		assert.NoError(t, err)
		assert.Equal(t, expected, count)
	}

	test([]int64{2}, []int64{1, 2, 3, 4}, 1)          // only userid 2
	test([]int64{1, 2, 3, 4, 5}, []int64{2, 5}, 2)    // userid 2,4
	test([]int64{1, 2, 3, 4, 5}, []int64{2, 3, 5}, 3) // userid 2,4,5
}
