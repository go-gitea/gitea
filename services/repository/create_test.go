// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository_test

import (
	"fmt"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/structs"
	org_service "code.gitea.io/gitea/services/org"
	repo_service "code.gitea.io/gitea/services/repository"

	"github.com/stretchr/testify/assert"
)

func TestIncludesAllRepositoriesTeams(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testTeamRepositories := func(teamID int64, repoIDs []int64) {
		team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID})
		repos, err := repo_model.GetTeamRepositories(db.DefaultContext, &repo_model.SearchTeamRepoOptions{
			TeamID: team.ID,
		})
		assert.NoError(t, err, "%s: GetRepositories", team.Name)
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
			assert.NoError(t, models.NewTeam(db.DefaultContext, team), "%s: NewTeam", team.Name)
		}
		testTeamRepositories(team.ID, teamRepos[i])
	}

	// Update teams and check repositories.
	teams[3].IncludesAllRepositories = false
	teams[4].IncludesAllRepositories = true
	teamRepos[4] = repoIDs
	for i, team := range teams {
		assert.NoError(t, models.UpdateTeam(db.DefaultContext, team, false, true), "%s: UpdateTeam", team.Name)
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
	assert.NoError(t, org_service.DeleteOrganization(db.DefaultContext, org, false), "DeleteOrganization")
}
