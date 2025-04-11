// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIRunnerOrgApi(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
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
}

func TestAPIRunnerDeleteReadScopeForbiddenOrgApi(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	userUsername := "user2"
	token := getUserToken(t, userUsername, auth_model.AccessTokenScopeReadOrganization)

	// Verify delete the runner by id is forbidden with read scope
	req := NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/orgs/org3/actions/runners/%d", 34347)).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusForbidden)
}

func TestAPIRunnerGetOrgApi(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
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
}

func TestAPIRunnerGetRepoScopeForbiddenOrgApi(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	userUsername := "user2"
	token := getUserToken(t, userUsername, auth_model.AccessTokenScopeReadRepository)
	// Verify get the runner by id with read scope
	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/orgs/org3/actions/runners/%d", 34347)).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusForbidden)
}

func TestAPIRunnerGetAdminRunnerNotFoundOrgApi(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	userUsername := "user2"
	token := getUserToken(t, userUsername, auth_model.AccessTokenScopeReadOrganization)
	// Verify get a runner by id of different entity is not found
	// runner.Editable(ownerID, repoID) false
	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/orgs/org3/actions/runners/%d", 34349)).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPIRunnerDeleteAdminRunnerNotFoundOrgApi(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	userUsername := "user2"
	token := getUserToken(t, userUsername, auth_model.AccessTokenScopeWriteOrganization)
	// Verify delete a runner by id of different entity is not found
	// runner.Editable(ownerID, repoID) false
	req := NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/orgs/org3/actions/runners/%d", 34349)).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPIRunnerDeleteNoConflictWhileJobIsDoneOrgApi(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	userUsername := "user2"
	token := getUserToken(t, userUsername, auth_model.AccessTokenScopeWriteOrganization)

	_, err := db.GetEngine(t.Context()).Insert(&actions_model.ActionTask{
		RunnerID: 34347,
		Status:   actions_model.StatusSuccess,
	})
	assert.NoError(t, err)

	// Verify delete the runner by id is ok
	req := NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/orgs/org3/actions/runners/%d", 34347)).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
}
