// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestUpdateRepoRunsNumbers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	assert.Equal(t, 4, repo.NumActionRuns)
	assert.Equal(t, 2, repo.NumClosedActionRuns)

	err := UpdateRepoRunsNumbers(t.Context(), repo)
	assert.NoError(t, err)
	repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	assert.Equal(t, 4, repo.NumActionRuns)
	assert.Equal(t, 3, repo.NumClosedActionRuns)
}
