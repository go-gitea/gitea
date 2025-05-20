// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"slices"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIActionsRunner(t *testing.T) {
	t.Run("AdminRunner", testActionsRunnerAdmin)
	t.Run("UserRunner", testActionsRunnerUser)
	t.Run("OwnerRunner", testActionsRunnerOwner)
	t.Run("RepoRunner", testActionsRunnerRepo)
}

func testActionsRunnerAdmin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	adminUsername := "user1"
	token := getUserToken(t, adminUsername, auth_model.AccessTokenScopeWriteAdmin)
	req := NewRequest(t, "POST", "/api/v1/admin/actions/runners/registration-token").AddTokenAuth(token)
	tokenResp := MakeRequest(t, req, http.StatusOK)
	var registrationToken struct {
		Token string `json:"token"`
	}
	DecodeJSON(t, tokenResp, &registrationToken)
	assert.NotEmpty(t, registrationToken.Token)

	req = NewRequest(t, "GET", "/api/v1/admin/actions/runners").AddTokenAuth(token)
	runnerListResp := MakeRequest(t, req, http.StatusOK)
	runnerList := api.ActionRunnersResponse{}
	DecodeJSON(t, runnerListResp, &runnerList)

	assert.Len(t, runnerList.Entries, 4)

	idx := slices.IndexFunc(runnerList.Entries, func(e *api.ActionRunner) bool { return e.ID == 34349 })
	require.NotEqual(t, -1, idx)
	expectedRunner := runnerList.Entries[idx]
	assert.Equal(t, "runner_to_be_deleted", expectedRunner.Name)
	assert.False(t, expectedRunner.Ephemeral)
	assert.Len(t, expectedRunner.Labels, 2)
	assert.Equal(t, "runner_to_be_deleted", expectedRunner.Labels[0].Name)
	assert.Equal(t, "linux", expectedRunner.Labels[1].Name)

	// Verify all returned runners can be requested and deleted
	for _, runnerEntry := range runnerList.Entries {
		// Verify get the runner by id
		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/admin/actions/runners/%d", runnerEntry.ID)).AddTokenAuth(token)
		runnerResp := MakeRequest(t, req, http.StatusOK)

		runner := api.ActionRunner{}
		DecodeJSON(t, runnerResp, &runner)

		assert.Equal(t, runnerEntry.Name, runner.Name)
		assert.Equal(t, runnerEntry.ID, runner.ID)
		assert.Equal(t, runnerEntry.Ephemeral, runner.Ephemeral)
		assert.ElementsMatch(t, runnerEntry.Labels, runner.Labels)

		req = NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/admin/actions/runners/%d", runnerEntry.ID)).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		// Verify runner deletion
		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/admin/actions/runners/%d", runnerEntry.ID)).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	}
}

func testActionsRunnerUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	userUsername := "user1"
	token := getUserToken(t, userUsername, auth_model.AccessTokenScopeWriteUser)
	req := NewRequest(t, "POST", "/api/v1/user/actions/runners/registration-token").AddTokenAuth(token)
	tokenResp := MakeRequest(t, req, http.StatusOK)
	var registrationToken struct {
		Token string `json:"token"`
	}
	DecodeJSON(t, tokenResp, &registrationToken)
	assert.NotEmpty(t, registrationToken.Token)

	req = NewRequest(t, "GET", "/api/v1/user/actions/runners").AddTokenAuth(token)
	runnerListResp := MakeRequest(t, req, http.StatusOK)
	runnerList := api.ActionRunnersResponse{}
	DecodeJSON(t, runnerListResp, &runnerList)

	assert.Len(t, runnerList.Entries, 1)
	assert.Equal(t, "runner_to_be_deleted-user", runnerList.Entries[0].Name)
	assert.Equal(t, int64(34346), runnerList.Entries[0].ID)
	assert.False(t, runnerList.Entries[0].Ephemeral)
	assert.Len(t, runnerList.Entries[0].Labels, 2)
	assert.Equal(t, "runner_to_be_deleted", runnerList.Entries[0].Labels[0].Name)
	assert.Equal(t, "linux", runnerList.Entries[0].Labels[1].Name)

	// Verify get the runner by id
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/user/actions/runners/%d", runnerList.Entries[0].ID)).AddTokenAuth(token)
	runnerResp := MakeRequest(t, req, http.StatusOK)

	runner := api.ActionRunner{}
	DecodeJSON(t, runnerResp, &runner)

	assert.Equal(t, "runner_to_be_deleted-user", runner.Name)
	assert.Equal(t, int64(34346), runner.ID)
	assert.False(t, runner.Ephemeral)
	assert.Len(t, runner.Labels, 2)
	assert.Equal(t, "runner_to_be_deleted", runner.Labels[0].Name)
	assert.Equal(t, "linux", runner.Labels[1].Name)

	// Verify delete the runner by id
	req = NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/user/actions/runners/%d", runnerList.Entries[0].ID)).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	// Verify runner deletion
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/user/actions/runners/%d", runnerList.Entries[0].ID)).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func testActionsRunnerOwner(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("GetRunner", func(t *testing.T) {
		userUsername := "user2"
		token := getUserToken(t, userUsername, auth_model.AccessTokenScopeReadOrganization)
		// Verify get the runner by id with read scope
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/orgs/org3/actions/runners/%d", 34347)).AddTokenAuth(token)
		runnerResp := MakeRequest(t, req, http.StatusOK)

		runner := api.ActionRunner{}
		DecodeJSON(t, runnerResp, &runner)

		assert.Equal(t, "runner_to_be_deleted-org", runner.Name)
		assert.Equal(t, int64(34347), runner.ID)
		assert.False(t, runner.Ephemeral)
		assert.Len(t, runner.Labels, 2)
		assert.Equal(t, "runner_to_be_deleted", runner.Labels[0].Name)
		assert.Equal(t, "linux", runner.Labels[1].Name)
	})

	t.Run("Access", func(t *testing.T) {
		userUsername := "user2"
		token := getUserToken(t, userUsername, auth_model.AccessTokenScopeWriteOrganization)
		req := NewRequest(t, "POST", "/api/v1/orgs/org3/actions/runners/registration-token").AddTokenAuth(token)
		tokenResp := MakeRequest(t, req, http.StatusOK)
		var registrationToken struct {
			Token string `json:"token"`
		}
		DecodeJSON(t, tokenResp, &registrationToken)
		assert.NotEmpty(t, registrationToken.Token)

		req = NewRequest(t, "GET", "/api/v1/orgs/org3/actions/runners").AddTokenAuth(token)
		runnerListResp := MakeRequest(t, req, http.StatusOK)
		runnerList := api.ActionRunnersResponse{}
		DecodeJSON(t, runnerListResp, &runnerList)

		assert.Len(t, runnerList.Entries, 1)
		assert.Equal(t, "runner_to_be_deleted-org", runnerList.Entries[0].Name)
		assert.Equal(t, int64(34347), runnerList.Entries[0].ID)
		assert.False(t, runnerList.Entries[0].Ephemeral)
		assert.Len(t, runnerList.Entries[0].Labels, 2)
		assert.Equal(t, "runner_to_be_deleted", runnerList.Entries[0].Labels[0].Name)
		assert.Equal(t, "linux", runnerList.Entries[0].Labels[1].Name)

		// Verify get the runner by id
		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/orgs/org3/actions/runners/%d", runnerList.Entries[0].ID)).AddTokenAuth(token)
		runnerResp := MakeRequest(t, req, http.StatusOK)

		runner := api.ActionRunner{}
		DecodeJSON(t, runnerResp, &runner)

		assert.Equal(t, "runner_to_be_deleted-org", runner.Name)
		assert.Equal(t, int64(34347), runner.ID)
		assert.False(t, runner.Ephemeral)
		assert.Len(t, runner.Labels, 2)
		assert.Equal(t, "runner_to_be_deleted", runner.Labels[0].Name)
		assert.Equal(t, "linux", runner.Labels[1].Name)

		// Verify delete the runner by id
		req = NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/orgs/org3/actions/runners/%d", runnerList.Entries[0].ID)).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		// Verify runner deletion
		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/orgs/org3/actions/runners/%d", runnerList.Entries[0].ID)).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("DeleteReadScopeForbidden", func(t *testing.T) {
		userUsername := "user2"
		token := getUserToken(t, userUsername, auth_model.AccessTokenScopeReadOrganization)

		// Verify delete the runner by id is forbidden with read scope
		req := NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/orgs/org3/actions/runners/%d", 34347)).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("GetRepoScopeForbidden", func(t *testing.T) {
		userUsername := "user2"
		token := getUserToken(t, userUsername, auth_model.AccessTokenScopeReadRepository)
		// Verify get the runner by id with read scope
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/orgs/org3/actions/runners/%d", 34347)).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("GetAdminRunner", func(t *testing.T) {
		userUsername := "user2"
		token := getUserToken(t, userUsername, auth_model.AccessTokenScopeReadOrganization)
		// Verify get a runner by id of different entity is not found
		// runner.EditableInContext(ownerID, repoID) false
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/orgs/org3/actions/runners/%d", 34349)).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("DeleteAdminRunner", func(t *testing.T) {
		userUsername := "user2"
		token := getUserToken(t, userUsername, auth_model.AccessTokenScopeWriteOrganization)
		// Verify delete a runner by id of different entity is not found
		// runner.EditableInContext(ownerID, repoID) false
		req := NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/orgs/org3/actions/runners/%d", 34349)).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})
}

func testActionsRunnerRepo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("GetRunner", func(t *testing.T) {
		userUsername := "user2"
		token := getUserToken(t, userUsername, auth_model.AccessTokenScopeReadRepository)
		// Verify get the runner by id with read scope
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/user2/repo1/actions/runners/%d", 34348)).AddTokenAuth(token)
		runnerResp := MakeRequest(t, req, http.StatusOK)

		runner := api.ActionRunner{}
		DecodeJSON(t, runnerResp, &runner)

		assert.Equal(t, "runner_to_be_deleted-repo1", runner.Name)
		assert.Equal(t, int64(34348), runner.ID)
		assert.False(t, runner.Ephemeral)
		assert.Len(t, runner.Labels, 2)
		assert.Equal(t, "runner_to_be_deleted", runner.Labels[0].Name)
		assert.Equal(t, "linux", runner.Labels[1].Name)
	})

	t.Run("Access", func(t *testing.T) {
		userUsername := "user2"
		token := getUserToken(t, userUsername, auth_model.AccessTokenScopeWriteRepository)
		req := NewRequest(t, "POST", "/api/v1/repos/user2/repo1/actions/runners/registration-token").AddTokenAuth(token)
		tokenResp := MakeRequest(t, req, http.StatusOK)
		var registrationToken struct {
			Token string `json:"token"`
		}
		DecodeJSON(t, tokenResp, &registrationToken)
		assert.NotEmpty(t, registrationToken.Token)

		req = NewRequest(t, "GET", "/api/v1/repos/user2/repo1/actions/runners").AddTokenAuth(token)
		runnerListResp := MakeRequest(t, req, http.StatusOK)
		runnerList := api.ActionRunnersResponse{}
		DecodeJSON(t, runnerListResp, &runnerList)

		assert.Len(t, runnerList.Entries, 1)
		assert.Equal(t, "runner_to_be_deleted-repo1", runnerList.Entries[0].Name)
		assert.Equal(t, int64(34348), runnerList.Entries[0].ID)
		assert.False(t, runnerList.Entries[0].Ephemeral)
		assert.Len(t, runnerList.Entries[0].Labels, 2)
		assert.Equal(t, "runner_to_be_deleted", runnerList.Entries[0].Labels[0].Name)
		assert.Equal(t, "linux", runnerList.Entries[0].Labels[1].Name)

		// Verify get the runner by id
		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/user2/repo1/actions/runners/%d", runnerList.Entries[0].ID)).AddTokenAuth(token)
		runnerResp := MakeRequest(t, req, http.StatusOK)

		runner := api.ActionRunner{}
		DecodeJSON(t, runnerResp, &runner)

		assert.Equal(t, "runner_to_be_deleted-repo1", runner.Name)
		assert.Equal(t, int64(34348), runner.ID)
		assert.False(t, runner.Ephemeral)
		assert.Len(t, runner.Labels, 2)
		assert.Equal(t, "runner_to_be_deleted", runner.Labels[0].Name)
		assert.Equal(t, "linux", runner.Labels[1].Name)

		// Verify delete the runner by id
		req = NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/repos/user2/repo1/actions/runners/%d", runnerList.Entries[0].ID)).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		// Verify runner deletion
		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/user2/repo1/actions/runners/%d", runnerList.Entries[0].ID)).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("DeleteReadScopeForbidden", func(t *testing.T) {
		userUsername := "user2"
		token := getUserToken(t, userUsername, auth_model.AccessTokenScopeReadRepository)

		// Verify delete the runner by id is forbidden with read scope
		req := NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/repos/user2/repo1/actions/runners/%d", 34348)).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("GetOrganizationScopeForbidden", func(t *testing.T) {
		userUsername := "user2"
		token := getUserToken(t, userUsername, auth_model.AccessTokenScopeReadOrganization)
		// Verify get the runner by id with read scope
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/user2/repo1/actions/runners/%d", 34348)).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("GetAdminRunnerNotFound", func(t *testing.T) {
		userUsername := "user2"
		token := getUserToken(t, userUsername, auth_model.AccessTokenScopeReadRepository)
		// Verify get a runner by id of different entity is not found
		// runner.EditableInContext(ownerID, repoID) false
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/user2/repo1/actions/runners/%d", 34349)).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("DeleteAdminRunnerNotFound", func(t *testing.T) {
		userUsername := "user2"
		token := getUserToken(t, userUsername, auth_model.AccessTokenScopeWriteRepository)
		// Verify delete a runner by id of different entity is not found
		// runner.EditableInContext(ownerID, repoID) false
		req := NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/repos/user2/repo1/actions/runners/%d", 34349)).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("DeleteAdminRunnerNotFoundUnknownID", func(t *testing.T) {
		userUsername := "user2"
		token := getUserToken(t, userUsername, auth_model.AccessTokenScopeWriteRepository)
		// Verify delete a runner by unknown id is not found
		req := NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/repos/user2/repo1/actions/runners/%d", 4384797347934)).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})
}
