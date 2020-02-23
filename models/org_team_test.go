// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestTeam_IsOwnerTeam(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	team := AssertExistsAndLoadBean(t, &Team{ID: 1}).(*Team)
	assert.True(t, team.IsOwnerTeam())

	team = AssertExistsAndLoadBean(t, &Team{ID: 2}).(*Team)
	assert.False(t, team.IsOwnerTeam())
}

func TestTeam_IsMember(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	team := AssertExistsAndLoadBean(t, &Team{ID: 1}).(*Team)
	assert.True(t, team.IsMember(2))
	assert.False(t, team.IsMember(4))
	assert.False(t, team.IsMember(NonexistentID))

	team = AssertExistsAndLoadBean(t, &Team{ID: 2}).(*Team)
	assert.True(t, team.IsMember(2))
	assert.True(t, team.IsMember(4))
	assert.False(t, team.IsMember(NonexistentID))
}

func TestTeam_GetRepositories(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	test := func(teamID int64) {
		team := AssertExistsAndLoadBean(t, &Team{ID: teamID}).(*Team)
		assert.NoError(t, team.GetRepositories())
		assert.Len(t, team.Repos, team.NumRepos)
		for _, repo := range team.Repos {
			AssertExistsAndLoadBean(t, &TeamRepo{TeamID: teamID, RepoID: repo.ID})
		}
	}
	test(1)
	test(3)
}

func TestTeam_GetMembers(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	test := func(teamID int64) {
		team := AssertExistsAndLoadBean(t, &Team{ID: teamID}).(*Team)
		assert.NoError(t, team.GetMembers())
		assert.Len(t, team.Members, team.NumMembers)
		for _, member := range team.Members {
			AssertExistsAndLoadBean(t, &TeamUser{UID: member.ID, TeamID: teamID})
		}
	}
	test(1)
	test(3)
}

func TestTeam_AddMember(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	test := func(teamID, userID int64) {
		team := AssertExistsAndLoadBean(t, &Team{ID: teamID}).(*Team)
		assert.NoError(t, team.AddMember(userID))
		AssertExistsAndLoadBean(t, &TeamUser{UID: userID, TeamID: teamID})
		CheckConsistencyFor(t, &Team{ID: teamID}, &User{ID: team.OrgID})
	}
	test(1, 2)
	test(1, 4)
	test(3, 2)
}

func TestTeam_RemoveMember(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	testSuccess := func(teamID, userID int64) {
		team := AssertExistsAndLoadBean(t, &Team{ID: teamID}).(*Team)
		assert.NoError(t, team.RemoveMember(userID))
		AssertNotExistsBean(t, &TeamUser{UID: userID, TeamID: teamID})
		CheckConsistencyFor(t, &Team{ID: teamID})
	}
	testSuccess(1, 4)
	testSuccess(2, 2)
	testSuccess(3, 2)
	testSuccess(3, NonexistentID)

	team := AssertExistsAndLoadBean(t, &Team{ID: 1}).(*Team)
	err := team.RemoveMember(2)
	assert.True(t, IsErrLastOrgOwner(err))
}

func TestTeam_HasRepository(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	test := func(teamID, repoID int64, expected bool) {
		team := AssertExistsAndLoadBean(t, &Team{ID: teamID}).(*Team)
		assert.Equal(t, expected, team.HasRepository(repoID))
	}
	test(1, 1, false)
	test(1, 3, true)
	test(1, 5, true)
	test(1, NonexistentID, false)

	test(2, 3, true)
	test(2, 5, false)
}

func TestTeam_AddRepository(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	testSuccess := func(teamID, repoID int64) {
		team := AssertExistsAndLoadBean(t, &Team{ID: teamID}).(*Team)
		repo := AssertExistsAndLoadBean(t, &Repository{ID: repoID}).(*Repository)
		assert.NoError(t, team.AddRepository(repo))
		AssertExistsAndLoadBean(t, &TeamRepo{TeamID: teamID, RepoID: repoID})
		CheckConsistencyFor(t, &Team{ID: teamID}, &Repository{ID: repoID})
	}
	testSuccess(2, 3)
	testSuccess(2, 5)

	team := AssertExistsAndLoadBean(t, &Team{ID: 1}).(*Team)
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	assert.Error(t, team.AddRepository(repo))
	CheckConsistencyFor(t, &Team{ID: 1}, &Repository{ID: 1})
}

func TestTeam_RemoveRepository(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	testSuccess := func(teamID, repoID int64) {
		team := AssertExistsAndLoadBean(t, &Team{ID: teamID}).(*Team)
		assert.NoError(t, team.RemoveRepository(repoID))
		AssertNotExistsBean(t, &TeamRepo{TeamID: teamID, RepoID: repoID})
		CheckConsistencyFor(t, &Team{ID: teamID}, &Repository{ID: repoID})
	}
	testSuccess(2, 3)
	testSuccess(2, 5)
	testSuccess(1, NonexistentID)
}

func TestIsUsableTeamName(t *testing.T) {
	assert.NoError(t, IsUsableTeamName("usable"))
	assert.True(t, IsErrNameReserved(IsUsableTeamName("new")))
}

func TestNewTeam(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	const teamName = "newTeamName"
	team := &Team{Name: teamName, OrgID: 3}
	assert.NoError(t, NewTeam(team))
	AssertExistsAndLoadBean(t, &Team{Name: teamName})
	CheckConsistencyFor(t, &Team{}, &User{ID: team.OrgID})
}

func TestGetTeam(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

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
	_, err = GetTeam(NonexistentID, "Owners")
	assert.Error(t, err)
}

func TestGetTeamByID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	testSuccess := func(teamID int64) {
		team, err := GetTeamByID(teamID)
		assert.NoError(t, err)
		assert.EqualValues(t, teamID, team.ID)
	}
	testSuccess(1)
	testSuccess(2)
	testSuccess(3)
	testSuccess(4)

	_, err := GetTeamByID(NonexistentID)
	assert.Error(t, err)
}

func TestUpdateTeam(t *testing.T) {
	// successful update
	assert.NoError(t, PrepareTestDatabase())

	team := AssertExistsAndLoadBean(t, &Team{ID: 2}).(*Team)
	team.LowerName = "newname"
	team.Name = "newName"
	team.Description = strings.Repeat("A long description!", 100)
	team.Authorize = AccessModeAdmin
	assert.NoError(t, UpdateTeam(team, true, false))

	team = AssertExistsAndLoadBean(t, &Team{Name: "newName"}).(*Team)
	assert.True(t, strings.HasPrefix(team.Description, "A long description!"))

	access := AssertExistsAndLoadBean(t, &Access{UserID: 4, RepoID: 3}).(*Access)
	assert.EqualValues(t, AccessModeAdmin, access.Mode)

	CheckConsistencyFor(t, &Team{ID: team.ID})
}

func TestUpdateTeam2(t *testing.T) {
	// update to already-existing team
	assert.NoError(t, PrepareTestDatabase())

	team := AssertExistsAndLoadBean(t, &Team{ID: 2}).(*Team)
	team.LowerName = "owners"
	team.Name = "Owners"
	team.Description = strings.Repeat("A long description!", 100)
	err := UpdateTeam(team, true, false)
	assert.True(t, IsErrTeamAlreadyExist(err))

	CheckConsistencyFor(t, &Team{ID: team.ID})
}

func TestDeleteTeam(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	team := AssertExistsAndLoadBean(t, &Team{ID: 2}).(*Team)
	assert.NoError(t, DeleteTeam(team))
	AssertNotExistsBean(t, &Team{ID: team.ID})
	AssertNotExistsBean(t, &TeamRepo{TeamID: team.ID})
	AssertNotExistsBean(t, &TeamUser{TeamID: team.ID})

	// check that team members don't have "leftover" access to repos
	user := AssertExistsAndLoadBean(t, &User{ID: 4}).(*User)
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 3}).(*Repository)
	accessMode, err := AccessLevel(user, repo)
	assert.NoError(t, err)
	assert.True(t, accessMode < AccessModeWrite)
}

func TestIsTeamMember(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	test := func(orgID, teamID, userID int64, expected bool) {
		isMember, err := IsTeamMember(orgID, teamID, userID)
		assert.NoError(t, err)
		assert.Equal(t, expected, isMember)
	}

	test(3, 1, 2, true)
	test(3, 1, 4, false)
	test(3, 1, NonexistentID, false)

	test(3, 2, 2, true)
	test(3, 2, 4, true)

	test(3, NonexistentID, NonexistentID, false)
	test(NonexistentID, NonexistentID, NonexistentID, false)
}

func TestGetTeamMembers(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	test := func(teamID int64) {
		team := AssertExistsAndLoadBean(t, &Team{ID: teamID}).(*Team)
		members, err := GetTeamMembers(teamID)
		assert.NoError(t, err)
		assert.Len(t, members, team.NumMembers)
		for _, member := range members {
			AssertExistsAndLoadBean(t, &TeamUser{UID: member.ID, TeamID: teamID})
		}
	}
	test(1)
	test(3)
}

func TestGetUserTeams(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	test := func(userID int64) {
		teams, err := GetUserTeams(userID)
		assert.NoError(t, err)
		for _, team := range teams {
			AssertExistsAndLoadBean(t, &TeamUser{TeamID: team.ID, UID: userID})
		}
	}
	test(2)
	test(5)
	test(NonexistentID)
}

func TestGetUserOrgTeams(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	test := func(orgID, userID int64) {
		teams, err := GetUserOrgTeams(orgID, userID)
		assert.NoError(t, err)
		for _, team := range teams {
			assert.EqualValues(t, orgID, team.OrgID)
			AssertExistsAndLoadBean(t, &TeamUser{TeamID: team.ID, UID: userID})
		}
	}
	test(3, 2)
	test(3, 4)
	test(3, NonexistentID)
}

func TestAddTeamMember(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	test := func(teamID, userID int64) {
		team := AssertExistsAndLoadBean(t, &Team{ID: teamID}).(*Team)
		assert.NoError(t, AddTeamMember(team, userID))
		AssertExistsAndLoadBean(t, &TeamUser{UID: userID, TeamID: teamID})
		CheckConsistencyFor(t, &Team{ID: teamID}, &User{ID: team.OrgID})
	}
	test(1, 2)
	test(1, 4)
	test(3, 2)
}

func TestRemoveTeamMember(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	testSuccess := func(teamID, userID int64) {
		team := AssertExistsAndLoadBean(t, &Team{ID: teamID}).(*Team)
		assert.NoError(t, RemoveTeamMember(team, userID))
		AssertNotExistsBean(t, &TeamUser{UID: userID, TeamID: teamID})
		CheckConsistencyFor(t, &Team{ID: teamID})
	}
	testSuccess(1, 4)
	testSuccess(2, 2)
	testSuccess(3, 2)
	testSuccess(3, NonexistentID)

	team := AssertExistsAndLoadBean(t, &Team{ID: 1}).(*Team)
	err := RemoveTeamMember(team, 2)
	assert.True(t, IsErrLastOrgOwner(err))
}

func TestHasTeamRepo(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	test := func(teamID, repoID int64, expected bool) {
		team := AssertExistsAndLoadBean(t, &Team{ID: teamID}).(*Team)
		assert.Equal(t, expected, HasTeamRepo(team.OrgID, teamID, repoID))
	}
	test(1, 1, false)
	test(1, 3, true)
	test(1, 5, true)
	test(1, NonexistentID, false)

	test(2, 3, true)
	test(2, 5, false)
}

func TestUsersInTeamsCount(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	test := func(teamIDs []int64, userIDs []int64, expected int64) {
		count, err := UsersInTeamsCount(teamIDs, userIDs)
		assert.NoError(t, err)
		assert.Equal(t, expected, count)
	}

	test([]int64{2}, []int64{1, 2, 3, 4}, 1)          // only userid 2
	test([]int64{1, 2, 3, 4, 5}, []int64{2, 5}, 2)    // userid 2,4
	test([]int64{1, 2, 3, 4, 5}, []int64{2, 3, 5}, 3) // userid 2,4,5
}

func TestIncludesAllRepositoriesTeams(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	testTeamRepositories := func(teamID int64, repoIds []int64) {
		team := AssertExistsAndLoadBean(t, &Team{ID: teamID}).(*Team)
		assert.NoError(t, team.GetRepositories(), "%s: GetRepositories", team.Name)
		assert.Len(t, team.Repos, team.NumRepos, "%s: len repo", team.Name)
		assert.Equal(t, len(repoIds), len(team.Repos), "%s: repo count", team.Name)
		for i, rid := range repoIds {
			if rid > 0 {
				assert.True(t, team.HasRepository(rid), "%s: HasRepository(%d) %d", rid, i)
			}
		}
	}

	// Get an admin user.
	user, err := GetUserByID(1)
	assert.NoError(t, err, "GetUserByID")

	// Create org.
	org := &User{
		Name:       "All_repo",
		IsActive:   true,
		Type:       UserTypeOrganization,
		Visibility: structs.VisibleTypePublic,
	}
	assert.NoError(t, CreateOrganization(org, user), "CreateOrganization")

	// Check Owner team.
	ownerTeam, err := org.GetOwnerTeam()
	assert.NoError(t, err, "GetOwnerTeam")
	assert.True(t, ownerTeam.IncludesAllRepositories, "Owner team includes all repositories")

	// Create repos.
	repoIds := make([]int64, 0)
	for i := 0; i < 3; i++ {
		r, err := CreateRepository(user, org, CreateRepoOptions{Name: fmt.Sprintf("repo-%d", i)})
		assert.NoError(t, err, "CreateRepository %d", i)
		if r != nil {
			repoIds = append(repoIds, r.ID)
		}
	}
	// Get fresh copy of Owner team after creating repos.
	ownerTeam, err = org.GetOwnerTeam()
	assert.NoError(t, err, "GetOwnerTeam")

	// Create teams and check repositories.
	teams := []*Team{
		ownerTeam,
		{
			OrgID:                   org.ID,
			Name:                    "team one",
			Authorize:               AccessModeRead,
			IncludesAllRepositories: true,
		},
		{
			OrgID:                   org.ID,
			Name:                    "team 2",
			Authorize:               AccessModeRead,
			IncludesAllRepositories: false,
		},
		{
			OrgID:                   org.ID,
			Name:                    "team three",
			Authorize:               AccessModeWrite,
			IncludesAllRepositories: true,
		},
		{
			OrgID:                   org.ID,
			Name:                    "team 4",
			Authorize:               AccessModeWrite,
			IncludesAllRepositories: false,
		},
	}
	teamRepos := [][]int64{
		repoIds,
		repoIds,
		{},
		repoIds,
		{},
	}
	for i, team := range teams {
		if i > 0 { // first team is Owner.
			assert.NoError(t, NewTeam(team), "%s: NewTeam", team.Name)
		}
		testTeamRepositories(team.ID, teamRepos[i])
	}

	// Update teams and check repositories.
	teams[3].IncludesAllRepositories = false
	teams[4].IncludesAllRepositories = true
	teamRepos[4] = repoIds
	for i, team := range teams {
		assert.NoError(t, UpdateTeam(team, false, true), "%s: UpdateTeam", team.Name)
		testTeamRepositories(team.ID, teamRepos[i])
	}

	// Create repo and check teams repositories.
	org.Teams = nil // Reset teams to allow their reloading.
	r, err := CreateRepository(user, org, CreateRepoOptions{Name: "repo-last"})
	assert.NoError(t, err, "CreateRepository last")
	if r != nil {
		repoIds = append(repoIds, r.ID)
	}
	teamRepos[0] = repoIds
	teamRepos[1] = repoIds
	teamRepos[4] = repoIds
	for i, team := range teams {
		testTeamRepositories(team.ID, teamRepos[i])
	}

	// Remove repo and check teams repositories.
	assert.NoError(t, DeleteRepository(user, org.ID, repoIds[0]), "DeleteRepository")
	teamRepos[0] = repoIds[1:]
	teamRepos[1] = repoIds[1:]
	teamRepos[3] = repoIds[1:3]
	teamRepos[4] = repoIds[1:]
	for i, team := range teams {
		testTeamRepositories(team.ID, teamRepos[i])
	}

	// Wipe created items.
	for i, rid := range repoIds {
		if i > 0 { // first repo already deleted.
			assert.NoError(t, DeleteRepository(user, org.ID, rid), "DeleteRepository %d", i)
		}
	}
	assert.NoError(t, DeleteOrganization(org), "DeleteOrganization")
}
