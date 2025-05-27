// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/util"
	repo_service "code.gitea.io/gitea/services/repository"
	user_service "code.gitea.io/gitea/services/user"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestEphemeralActionsRunnerDeletion(t *testing.T) {
	t.Run("ByTaskCompletion", testEphemeralActionsRunnerDeletionByTaskCompletion)
	t.Run("ByRepository", testEphemeralActionsRunnerDeletionByRepository)
	t.Run("ByUser", testEphemeralActionsRunnerDeletionByUser)
}

// Test that the ephemeral runner is deleted when the task is finished
func testEphemeralActionsRunnerDeletionByTaskCompletion(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	_, err := actions_model.GetRunnerByID(t.Context(), 34350)
	assert.NoError(t, err)

	task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 52})
	assert.Equal(t, actions_model.StatusRunning, task.Status)

	task.Status = actions_model.StatusSuccess
	err = actions_model.UpdateTask(t.Context(), task, "status")
	assert.NoError(t, err)

	_, err = actions_model.GetRunnerByID(t.Context(), 34350)
	assert.ErrorIs(t, err, util.ErrNotExist)
}

func testEphemeralActionsRunnerDeletionByRepository(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	_, err := actions_model.GetRunnerByID(t.Context(), 34350)
	assert.NoError(t, err)

	task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 52})
	assert.Equal(t, actions_model.StatusRunning, task.Status)

	err = repo_service.DeleteRepositoryDirectly(t.Context(), task.RepoID, true)
	assert.NoError(t, err)

	_, err = actions_model.GetRunnerByID(t.Context(), 34350)
	assert.ErrorIs(t, err, util.ErrNotExist)
}

// Test that the ephemeral runner is deleted when a user is deleted
func testEphemeralActionsRunnerDeletionByUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	_, err := actions_model.GetRunnerByID(t.Context(), 34350)
	assert.NoError(t, err)

	task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 52})
	assert.Equal(t, actions_model.StatusRunning, task.Status)

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	err = user_service.DeleteUser(t.Context(), user, true)
	assert.NoError(t, err)

	_, err = actions_model.GetRunnerByID(t.Context(), 34350)
	assert.ErrorIs(t, err, util.ErrNotExist)
}
