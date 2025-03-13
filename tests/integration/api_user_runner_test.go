// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"
	"github.com/stretchr/testify/assert"
)

func TestAPIRunnerUserApi(t *testing.T) {
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
	assert.Equal(t, false, runnerList.Entries[0].Ephemeral)
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
	assert.Equal(t, false, runner.Ephemeral)
	assert.Len(t, runner.Labels, 2)
	assert.Equal(t, "runner_to_be_deleted", runner.Labels[0].Name)
	assert.Equal(t, "linux", runner.Labels[1].Name)

	// Verify delete the runner by id
	req = NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/user/actions/runners/%d", runnerList.Entries[0].ID)).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	// Verify get the runner has been deleted
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/user/actions/runners/%d", runnerList.Entries[0].ID)).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}
