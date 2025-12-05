// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/tests"
	"github.com/stretchr/testify/assert"
)

func TestActionUserSignIn(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/api/v1/user").
		AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	resp := MakeRequest(t, req, http.StatusOK)

	var u api.User
	DecodeJSON(t, resp, &u)
	assert.Equal(t, "gitea-actions", u.UserName)
}

func TestActionUserAccessPublicRepo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequestf(t, "GET", "/api/v1/repos/user2/repo1/raw/README.md").
		AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	resp := MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "file", resp.Header().Get("x-gitea-object-type"))

	defer test.MockVariableValue(&setting.Service.RequireSignInViewStrict, true)()

	req = NewRequestf(t, "GET", "/api/v1/repos/user2/repo1/raw/README.md").
		AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	resp = MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "file", resp.Header().Get("x-gitea-object-type"))
}

func TestActionUserNoAccessOtherPrivateRepo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequestf(t, "GET", "/api/v1/repos/user2/repo2/raw/README.md").
		AddTokenAuth("8061e833a55f6fc0157c98b883e91fcfeeb1a71a")
	MakeRequest(t, req, http.StatusNotFound)
}
