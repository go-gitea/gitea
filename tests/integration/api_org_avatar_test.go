// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"net/http"
	"os"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIUpdateOrgAvatar(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")

	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteOrganization)

	avatar, err := os.ReadFile("tests/integration/avatar.png")
	assert.NoError(t, err)
	if err != nil {
		assert.FailNow(t, "Unable to open avatar.png")
	}

	opts := api.UpdateUserAvatarOption{
		Image: base64.StdEncoding.EncodeToString(avatar),
	}

	req := NewRequestWithJSON(t, "POST", "/api/v1/orgs/user3/avatar?token="+token, &opts)
	MakeRequest(t, req, http.StatusNoContent)

	opts = api.UpdateUserAvatarOption{
		Image: "Invalid",
	}

	req = NewRequestWithJSON(t, "POST", "/api/v1/orgs/user3/avatar?token="+token, &opts)
	MakeRequest(t, req, http.StatusBadRequest)
}

func TestAPIDeleteOrgAvatar(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")

	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteOrganization)

	req := NewRequest(t, "DELETE", "/api/v1/orgs/user3/avatar?token="+token)
	MakeRequest(t, req, http.StatusNoContent)
}
