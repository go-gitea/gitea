// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestRepositoryReparent(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	source := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	target := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 11})

	// Start reparent (target owner is same as source owner in this fixture? No, repo 10 is user12, 11 is user13? Let's check)
	// Repo 10 owner is 12. Repo 11 owner is 13.
	targetOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 13})

	sourceAfter, err := StartRepositoryReparent(t.Context(), doer, source, targetOwner.ID)
	assert.NoError(t, err)
	assert.Equal(t, repo_model.RepositoryPendingReparent, sourceAfter.Status)

	// Cancel reparent
	err = CancelRepositoryReparent(t.Context(), doer, sourceAfter)
	assert.NoError(t, err)

	sourceAfterCancel, err := repo_model.GetRepositoryByID(t.Context(), source.ID)
	assert.NoError(t, err)
	assert.Equal(t, repo_model.RepositoryReady, sourceAfterCancel.Status)

	// Start reparent again to test accept
	sourceBeforeAccept, err := StartRepositoryReparent(t.Context(), doer, sourceAfterCancel, targetOwner.ID)
	assert.NoError(t, err)

	// Accept reparent
	err = AcceptReparent(t.Context(), doer, sourceBeforeAccept)
	assert.NoError(t, err)

	sourceFinal, err := repo_model.GetRepositoryByID(t.Context(), source.ID)
	assert.NoError(t, err)
	assert.Equal(t, repo_model.RepositoryReady, sourceFinal.Status)
	assert.True(t, sourceFinal.IsFork)
	assert.Equal(t, target.ID, sourceFinal.ForkID)

	targetFinal, err := repo_model.GetRepositoryByID(t.Context(), target.ID)
	assert.NoError(t, err)
	assert.False(t, targetFinal.IsFork)
	assert.Equal(t, int64(0), targetFinal.ForkID)
}
