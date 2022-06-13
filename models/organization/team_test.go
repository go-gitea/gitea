// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package organization

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

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
		assert.NoError(t, team.GetRepositoriesCtx(db.DefaultContext))
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
		assert.NoError(t, team.GetMembersCtx(db.DefaultContext))
		assert.Len(t, team.Members, team.NumMembers)
		for _, member := range team.Members {
			unittest.AssertExistsAndLoadBean(t, &TeamUser{UID: member.ID, TeamID: teamID})
		}
	}
	test(1)
	test(3)
}

func TestGetTeam(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(orgID int64, name string) {
		team, err := GetTeam(db.DefaultContext, orgID, name)
		assert.NoError(t, err)
		assert.EqualValues(t, orgID, team.OrgID)
		assert.Equal(t, name, team.Name)
	}
	testSuccess(3, "Owners")
	testSuccess(3, "team1")

	_, err := GetTeam(db.DefaultContext, 3, "nonexistent")
	assert.Error(t, err)
	_, err = GetTeam(db.DefaultContext, unittest.NonexistentID, "Owners")
	assert.Error(t, err)
}

func TestGetTeamByID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(teamID int64) {
		team, err := GetTeamByID(db.DefaultContext, teamID)
		assert.NoError(t, err)
		assert.EqualValues(t, teamID, team.ID)
	}
	testSuccess(1)
	testSuccess(2)
	testSuccess(3)
	testSuccess(4)

	_, err := GetTeamByID(db.DefaultContext, unittest.NonexistentID)
	assert.Error(t, err)
}

func TestIsTeamMember(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	test := func(orgID, teamID, userID int64, expected bool) {
		isMember, err := IsTeamMember(db.DefaultContext, orgID, teamID, userID)
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
		members, err := GetTeamMembers(db.DefaultContext, &SearchMembersOptions{
			TeamID: teamID,
		})
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
		teams, _, err := SearchTeam(&SearchTeamOptions{UserID: userID})
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
		teams, err := GetUserOrgTeams(db.DefaultContext, orgID, userID)
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

func TestHasTeamRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(teamID, repoID int64, expected bool) {
		team := unittest.AssertExistsAndLoadBean(t, &Team{ID: teamID}).(*Team)
		assert.Equal(t, expected, HasTeamRepo(db.DefaultContext, team.OrgID, teamID, repoID))
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
