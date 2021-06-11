// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"fmt"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestIncludesAllRepositoriesTeams(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())

	testTeamRepositories := func(teamID int64, repoIds []int64) {
		team := models.AssertExistsAndLoadBean(t, &models.Team{ID: teamID}).(*models.Team)
		assert.NoError(t, team.GetRepositories(&models.SearchTeamOptions{}), "%s: GetRepositories", team.Name)
		assert.Len(t, team.Repos, team.NumRepos, "%s: len repo", team.Name)
		assert.Len(t, team.Repos, len(repoIds), "%s: repo count", team.Name)
		for i, rid := range repoIds {
			if rid > 0 {
				assert.True(t, team.HasRepository(rid), "%s: HasRepository(%d) %d", rid, i)
			}
		}
	}

	// Get an admin user.
	user, err := models.GetUserByID(1)
	assert.NoError(t, err, "GetUserByID")

	// Create org.
	org := &models.User{
		Name:       "All_repo",
		IsActive:   true,
		Type:       models.UserTypeOrganization,
		Visibility: structs.VisibleTypePublic,
	}
	assert.NoError(t, models.CreateOrganization(org, user), "CreateOrganization")

	// Check Owner team.
	ownerTeam, err := org.GetOwnerTeam()
	assert.NoError(t, err, "GetOwnerTeam")
	assert.True(t, ownerTeam.IncludesAllRepositories, "Owner team includes all repositories")

	// Create repos.
	repoIds := make([]int64, 0)
	for i := 0; i < 3; i++ {
		r, err := CreateRepository(user, org, models.CreateRepoOptions{Name: fmt.Sprintf("repo-%d", i)})
		assert.NoError(t, err, "CreateRepository %d", i)
		if r != nil {
			repoIds = append(repoIds, r.ID)
		}
	}
	// Get fresh copy of Owner team after creating repos.
	ownerTeam, err = org.GetOwnerTeam()
	assert.NoError(t, err, "GetOwnerTeam")

	// Create teams and check repositories.
	teams := []*models.Team{
		ownerTeam,
		{
			OrgID:                   org.ID,
			Name:                    "team one",
			Authorize:               models.AccessModeRead,
			IncludesAllRepositories: true,
		},
		{
			OrgID:                   org.ID,
			Name:                    "team 2",
			Authorize:               models.AccessModeRead,
			IncludesAllRepositories: false,
		},
		{
			OrgID:                   org.ID,
			Name:                    "team three",
			Authorize:               models.AccessModeWrite,
			IncludesAllRepositories: true,
		},
		{
			OrgID:                   org.ID,
			Name:                    "team 4",
			Authorize:               models.AccessModeWrite,
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
			assert.NoError(t, models.NewTeam(team), "%s: NewTeam", team.Name)
		}
		testTeamRepositories(team.ID, teamRepos[i])
	}

	// Update teams and check repositories.
	teams[3].IncludesAllRepositories = false
	teams[4].IncludesAllRepositories = true
	teamRepos[4] = repoIds
	for i, team := range teams {
		assert.NoError(t, models.UpdateTeam(team, false, true), "%s: UpdateTeam", team.Name)
		testTeamRepositories(team.ID, teamRepos[i])
	}

	// Create repo and check teams repositories.
	org.Teams = nil // Reset teams to allow their reloading.
	r, err := CreateRepository(user, org, models.CreateRepoOptions{Name: "repo-last"})
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
	assert.NoError(t, models.DeleteRepository(user, org.ID, repoIds[0]), "DeleteRepository")
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
			assert.NoError(t, models.DeleteRepository(user, org.ID, rid), "DeleteRepository %d", i)
		}
	}
	assert.NoError(t, models.DeleteOrganization(org), "DeleteOrganization")
}
