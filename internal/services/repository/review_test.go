// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	"code.gitea.io/gitea/internal/models/db"
	repo_model "code.gitea.io/gitea/internal/models/repo"
	"code.gitea.io/gitea/internal/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestRepoGetReviewerTeams(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	teams, err := GetReviewerTeams(db.DefaultContext, repo2)
	assert.NoError(t, err)
	assert.Empty(t, teams)

	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	teams, err = GetReviewerTeams(db.DefaultContext, repo3)
	assert.NoError(t, err)
	assert.Len(t, teams, 2)
}
