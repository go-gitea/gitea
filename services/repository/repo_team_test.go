// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	advisory_model "gitea.dev/models/advisory"
	"gitea.dev/models/organization"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTeam_AddRepository(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(teamID, repoID int64) {
		team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID})
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
		assert.NoError(t, TeamAddRepository(t.Context(), team, repo))
		unittest.AssertExistsAndLoadBean(t, &organization.TeamRepo{TeamID: teamID, RepoID: repoID})
		unittest.CheckConsistencyFor(t, &organization.Team{ID: teamID}, &repo_model.Repository{ID: repoID})
	}
	testSuccess(2, 3)
	testSuccess(2, 5)

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 1})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.Error(t, TeamAddRepository(t.Context(), team, repo))
	unittest.CheckConsistencyFor(t, &organization.Team{ID: 1}, &repo_model.Repository{ID: 1})
}

func TestTeam_RemoveRepository(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// team 2 has the private repository 3
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	a := &advisory_model.Advisory{RepoID: 3, Summary: "s", State: advisory_model.StateDraft, ReporterID: 2}
	require.NoError(t, advisory_model.CreateAdvisory(ctx, a))
	for _, c := range []*advisory_model.Collaborator{{TeamID: 2}, {TeamID: 1}, {UserID: 4}} {
		c.AdvisoryID = a.ID
		_, err := advisory_model.AddCollaborator(ctx, c)
		require.NoError(t, err)
	}

	require.NoError(t, RemoveRepositoryFromTeam(ctx, team, 3))
	unittest.AssertNotExistsBean(t, &advisory_model.Collaborator{AdvisoryID: a.ID, TeamID: 2})
	unittest.AssertCount(t, &advisory_model.Collaborator{AdvisoryID: a.ID}, 2)
}
