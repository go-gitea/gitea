// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestGetUserFork(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// User13 has repo 11 forked from repo10
	repo, err := repo_model.GetRepositoryByID(t.Context(), 10)
	assert.NoError(t, err)
	assert.NotNil(t, repo)
	repo, err = repo_model.GetUserFork(t.Context(), repo.ID, 13)
	assert.NoError(t, err)
	assert.NotNil(t, repo)

	repo, err = repo_model.GetRepositoryByID(t.Context(), 9)
	assert.NoError(t, err)
	assert.NotNil(t, repo)
	repo, err = repo_model.GetUserFork(t.Context(), repo.ID, 13)
	assert.NoError(t, err)
	assert.Nil(t, repo)
}
