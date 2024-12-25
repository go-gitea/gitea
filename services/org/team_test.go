// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"fmt"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/structs"
	repo_service "code.gitea.io/gitea/services/repository"

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
	assert.Less(t, accessMode, perm.AccessModeWrite)
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

func TestIncludesAllRepositoriesTeams(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testTeamRepositories := func(teamID int64, repoIDs []int64) {
		team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID})
		repos, err := repo_model.GetTeamRepositories(db.DefaultContext, &repo_model.SearchTeamRepoOptions{
			TeamID: team.ID,
		})
		assert.NoError(t, err, "%s: GetTeamRepositories", team.Name)
		assert.Len(t, repos, team.NumRepos, "%s: len repo", team.Name)
		assert.Len(t, repos, len(repoIDs), "%s: repo count", team.Name)
		for i, rid := range repoIDs {
			if rid > 0 {
				assert.True(t, repo_service.HasRepository(db.DefaultContext, team, rid), "%s: HasRepository(%d) %d", rid, i)
			}
		}
	}

	// Get an admin user.
	user, err := user_model.GetUserByID(db.DefaultContext, 1)
	assert.NoError(t, err, "GetUserByID")

	// Create org.
	org := &organization.Organization{
		Name:       "All_repo",
		IsActive:   true,
		Type:       user_model.UserTypeOrganization,
		Visibility: structs.VisibleTypePublic,
	}
	assert.NoError(t, organization.CreateOrganization(db.DefaultContext, org, user), "CreateOrganization")

	// Check Owner team.
	ownerTeam, err := org.GetOwnerTeam(db.DefaultContext)
	assert.NoError(t, err, "GetOwnerTeam")
	assert.True(t, ownerTeam.IncludesAllRepositories, "Owner team includes all repositories")

	// Create repos.
	repoIDs := make([]int64, 0)
	for i := 0; i < 3; i++ {
		r, err := repo_service.CreateRepositoryDirectly(db.DefaultContext, user, org.AsUser(), repo_service.CreateRepoOptions{Name: fmt.Sprintf("repo-%d", i)})
		assert.NoError(t, err, "CreateRepository %d", i)
		if r != nil {
			repoIDs = append(repoIDs, r.ID)
		}
	}
	// Get fresh copy of Owner team after creating repos.
	ownerTeam, err = org.GetOwnerTeam(db.DefaultContext)
	assert.NoError(t, err, "GetOwnerTeam")

	// Create teams and check repositories.
	teams := []*organization.Team{
		ownerTeam,
		{
			OrgID:                   org.ID,
			Name:                    "team one",
			AccessMode:              perm.AccessModeRead,
			IncludesAllRepositories: true,
		},
		{
			OrgID:                   org.ID,
			Name:                    "team 2",
			AccessMode:              perm.AccessModeRead,
			IncludesAllRepositories: false,
		},
		{
			OrgID:                   org.ID,
			Name:                    "team three",
			AccessMode:              perm.AccessModeWrite,
			IncludesAllRepositories: true,
		},
		{
			OrgID:                   org.ID,
			Name:                    "team 4",
			AccessMode:              perm.AccessModeWrite,
			IncludesAllRepositories: false,
		},
	}
	teamRepos := [][]int64{
		repoIDs,
		repoIDs,
		{},
		repoIDs,
		{},
	}
	for i, team := range teams {
		if i > 0 { // first team is Owner.
			assert.NoError(t, NewTeam(db.DefaultContext, team), "%s: NewTeam", team.Name)
		}
		testTeamRepositories(team.ID, teamRepos[i])
	}

	// Update teams and check repositories.
	teams[3].IncludesAllRepositories = false
	teams[4].IncludesAllRepositories = true
	teamRepos[4] = repoIDs
	for i, team := range teams {
		assert.NoError(t, UpdateTeam(db.DefaultContext, team, false, true), "%s: UpdateTeam", team.Name)
		testTeamRepositories(team.ID, teamRepos[i])
	}

	// Create repo and check teams repositories.
	r, err := repo_service.CreateRepositoryDirectly(db.DefaultContext, user, org.AsUser(), repo_service.CreateRepoOptions{Name: "repo-last"})
	assert.NoError(t, err, "CreateRepository last")
	if r != nil {
		repoIDs = append(repoIDs, r.ID)
	}
	teamRepos[0] = repoIDs
	teamRepos[1] = repoIDs
	teamRepos[4] = repoIDs
	for i, team := range teams {
		testTeamRepositories(team.ID, teamRepos[i])
	}

	// Remove repo and check teams repositories.
	assert.NoError(t, repo_service.DeleteRepositoryDirectly(db.DefaultContext, user, repoIDs[0]), "DeleteRepository")
	teamRepos[0] = repoIDs[1:]
	teamRepos[1] = repoIDs[1:]
	teamRepos[3] = repoIDs[1:3]
	teamRepos[4] = repoIDs[1:]
	for i, team := range teams {
		testTeamRepositories(team.ID, teamRepos[i])
	}

	// Wipe created items.
	for i, rid := range repoIDs {
		if i > 0 { // first repo already deleted.
			assert.NoError(t, repo_service.DeleteRepositoryDirectly(db.DefaultContext, user, rid), "DeleteRepository %d", i)
		}
	}
	assert.NoError(t, DeleteOrganization(db.DefaultContext, org, false), "DeleteOrganization")
}
