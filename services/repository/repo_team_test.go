// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestTeam_AddRepository(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(teamID, repoID int64) {
		team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID})
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
		assert.NoError(t, TeamAddRepository(db.DefaultContext, team, repo))
		unittest.AssertExistsAndLoadBean(t, &organization.TeamRepo{TeamID: teamID, RepoID: repoID})
		unittest.CheckConsistencyFor(t, &organization.Team{ID: teamID}, &repo_model.Repository{ID: repoID})
	}
	testSuccess(2, 3)
	testSuccess(2, 5)

	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 1})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.Error(t, TeamAddRepository(db.DefaultContext, team, repo))
	unittest.CheckConsistencyFor(t, &organization.Team{ID: 1}, &repo_model.Repository{ID: 1})
}
