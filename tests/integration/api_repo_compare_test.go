// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"log"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPICompareTag(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	// Login as User2.
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	repoName := "repo1"

	req := NewRequestf(t, "GET", "/api/v1/repos/user2/%s/compare/v1.1...master", repoName).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	var apiResp *api.Compare
	DecodeJSON(t, resp, &apiResp)

	log.Printf("Total commits: %v", apiResp.TotalCommits)
	log.Printf("Commits: %v", apiResp.Commits)
	assert.Len(t, apiResp.TotalCommits, 1)
}
