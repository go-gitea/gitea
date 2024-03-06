// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestUpdateRepositoryVisibilityChanged(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Get sample repo and change visibility
	repo, err := repo_model.GetRepositoryByID(db.DefaultContext, 9)
	assert.NoError(t, err)
	repo.IsPrivate = true

	// Update it
	err = UpdateRepository(db.DefaultContext, repo, true)
	assert.NoError(t, err)

	// Check visibility of action has become private
	act := activities_model.Action{}
	_, err = db.GetEngine(db.DefaultContext).ID(3).Get(&act)

	assert.NoError(t, err)
	assert.True(t, act.IsPrivate)
}

func TestGetDirectorySize(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo, err := repo_model.GetRepositoryByID(db.DefaultContext, 1)
	assert.NoError(t, err)

	size, err := getDirectorySize(repo.RepoPath())
	assert.NoError(t, err)
	assert.EqualValues(t, size, repo.Size)
}
