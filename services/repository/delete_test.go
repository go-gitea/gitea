// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	repo_service "code.gitea.io/gitea/services/repository"

	"github.com/stretchr/testify/assert"
)

func TestTeam_HasRepository(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(teamID, repoID int64, expected bool) {
		team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID})
		assert.Equal(t, expected, repo_service.HasRepository(db.DefaultContext, team, repoID))
	}
	test(1, 1, false)
	test(1, 3, true)
	test(1, 5, true)
	test(1, unittest.NonexistentID, false)

	test(2, 3, true)
	test(2, 5, false)
}

func TestTeam_RemoveRepository(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(teamID, repoID int64) {
		team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID})
		assert.NoError(t, repo_service.RemoveRepositoryFromTeam(db.DefaultContext, team, repoID))
		unittest.AssertNotExistsBean(t, &organization.TeamRepo{TeamID: teamID, RepoID: repoID})
		unittest.CheckConsistencyFor(t, &organization.Team{ID: teamID}, &repo_model.Repository{ID: repoID})
	}
	testSuccess(2, 3)
	testSuccess(2, 5)
	testSuccess(1, unittest.NonexistentID)
}

func TestDeleteOwnerRepositoriesDirectly(t *testing.T) {
	unittest.PrepareTestEnv(t)

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	assert.NoError(t, repo_service.DeleteOwnerRepositoriesDirectly(db.DefaultContext, user))
}
