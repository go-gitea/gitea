// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"fmt"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestIncludesAllRepositoriesTeams(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testTeamRepositories := func(teamID int64, repoIds []int64) {
		team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID})
		assert.NoError(t, team.LoadRepositories(db.DefaultContext), "%s: GetRepositories", team.Name)
		assert.Len(t, team.Repos, team.NumRepos, "%s: len repo", team.Name)
		assert.Len(t, team.Repos, len(repoIds), "%s: repo count", team.Name)
		for i, rid := range repoIds {
			if rid > 0 {
				assert.True(t, HasRepository(db.DefaultContext, team, rid), "%s: HasRepository(%d) %d", rid, i)
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
	repoIds := make([]int64, 0)
	for i := 0; i < 3; i++ {
		r, err := CreateRepositoryDirectly(db.DefaultContext, user, org.AsUser(), CreateRepoOptions{Name: fmt.Sprintf("repo-%d", i)})
		assert.NoError(t, err, "CreateRepository %d", i)
		if r != nil {
			repoIds = append(repoIds, r.ID)
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
		repoIds,
		repoIds,
		{},
		repoIds,
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
	teamRepos[4] = repoIds
	for i, team := range teams {
		assert.NoError(t, models.UpdateTeam(db.DefaultContext, team, false, true), "%s: UpdateTeam", team.Name)
		testTeamRepositories(team.ID, teamRepos[i])
	}

	// Create repo and check teams repositories.
	r, err := CreateRepositoryDirectly(db.DefaultContext, user, org.AsUser(), CreateRepoOptions{Name: "repo-last"})
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
	assert.NoError(t, DeleteRepositoryDirectly(db.DefaultContext, user, repoIds[0]), "DeleteRepository")
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
			assert.NoError(t, DeleteRepositoryDirectly(db.DefaultContext, user, rid), "DeleteRepository %d", i)
		}
	}
	assert.NoError(t, organization.DeleteOrganization(db.DefaultContext, org), "DeleteOrganization")
}
