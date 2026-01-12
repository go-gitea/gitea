// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIUpdateRunnerCapacity(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	ctx := context.Background()

	// Clean up existing runners
	require.NoError(t, db.DeleteAllRecords("action_runner"))

	// Create a test runner
	runner := &actions_model.ActionRunner{
		UUID:      "test-capacity-runner",
		Name:      "Test Capacity Runner",
		OwnerID:   0,
		RepoID:    0,
		Capacity:  1,
		TokenHash: "test-capacity-hash",
		Token:     "test-capacity-token",
	}
	require.NoError(t, actions_model.CreateRunner(ctx, runner))

	// Load the created runner to get the ID
	runner = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunner{UUID: "test-capacity-runner"})

	session := loginUser(t, "user1")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteAdmin)

	t.Run("UpdateCapacity", func(t *testing.T) {
		req := NewRequestWithJSON(t, "PATCH",
			fmt.Sprintf("/api/v1/admin/actions/runners/%d/capacity", runner.ID),
			&api.UpdateRunnerCapacityOption{Capacity: 5}).
			AddTokenAuth(token)

		resp := MakeRequest(t, req, http.StatusOK)

		var apiRunner api.ActionRunner
		DecodeJSON(t, resp, &apiRunner)

		assert.Equal(t, runner.ID, apiRunner.ID)
		assert.Equal(t, 5, apiRunner.Capacity)

		// Verify in database
		updated, err := actions_model.GetRunnerByID(context.Background(), runner.ID)
		assert.NoError(t, err)
		assert.Equal(t, 5, updated.Capacity)
	})

	t.Run("UpdateCapacityToZero", func(t *testing.T) {
		req := NewRequestWithJSON(t, "PATCH",
			fmt.Sprintf("/api/v1/admin/actions/runners/%d/capacity", runner.ID),
			&api.UpdateRunnerCapacityOption{Capacity: 0}).
			AddTokenAuth(token)

		resp := MakeRequest(t, req, http.StatusOK)

		var apiRunner api.ActionRunner
		DecodeJSON(t, resp, &apiRunner)

		assert.Equal(t, 0, apiRunner.Capacity)
	})

	t.Run("InvalidCapacity", func(t *testing.T) {
		req := NewRequestWithJSON(t, "PATCH",
			fmt.Sprintf("/api/v1/admin/actions/runners/%d/capacity", runner.ID),
			&api.UpdateRunnerCapacityOption{Capacity: -1}).
			AddTokenAuth(token)

		MakeRequest(t, req, http.StatusBadRequest)
	})

	t.Run("NonExistentRunner", func(t *testing.T) {
		req := NewRequestWithJSON(t, "PATCH",
			"/api/v1/admin/actions/runners/999999/capacity",
			&api.UpdateRunnerCapacityOption{Capacity: 5}).
			AddTokenAuth(token)

		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("GetRunnerWithCapacity", func(t *testing.T) {
		// First set capacity
		runner.Capacity = 7
		assert.NoError(t, actions_model.UpdateRunner(context.Background(), runner, "capacity"))

		// Get runner
		req := NewRequest(t, "GET",
			fmt.Sprintf("/api/v1/admin/actions/runners/%d", runner.ID)).
			AddTokenAuth(token)

		resp := MakeRequest(t, req, http.StatusOK)

		var apiRunner api.ActionRunner
		DecodeJSON(t, resp, &apiRunner)

		assert.Equal(t, runner.ID, apiRunner.ID)
		assert.Equal(t, 7, apiRunner.Capacity)
	})

	t.Run("ListRunnersWithCapacity", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/admin/actions/runners").
			AddTokenAuth(token)

		resp := MakeRequest(t, req, http.StatusOK)

		var response api.ActionRunnersResponse
		DecodeJSON(t, resp, &response)

		// Find our test runner
		found := false
		for _, r := range response.Entries {
			if r.ID == runner.ID {
				found = true
				assert.Equal(t, 7, r.Capacity)
				break
			}
		}
		assert.True(t, found, "Test runner should be in list")
	})

	t.Run("UnauthorizedAccess", func(t *testing.T) {
		req := NewRequestWithJSON(t, "PATCH",
			fmt.Sprintf("/api/v1/admin/actions/runners/%d/capacity", runner.ID),
			&api.UpdateRunnerCapacityOption{Capacity: 5})
		// No token

		MakeRequest(t, req, http.StatusUnauthorized)
	})
}
