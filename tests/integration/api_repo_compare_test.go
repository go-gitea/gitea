// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPICompareBranches(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	// Login as User2.
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	t.Run("CompareBranches", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequestf(t, "GET", "/api/v1/repos/user2/repo20/compare/add-csv...remove-files-b").AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		var apiResp *api.Compare
		DecodeJSON(t, resp, &apiResp)

		assert.Equal(t, 2, apiResp.TotalCommits)
		assert.Len(t, apiResp.Commits, 2)
	})

	t.Run("CompareCommits", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequestf(t, "GET", "/api/v1/repos/user2/repo20/compare/808038d2f71b0ab02099...c8e31bc7688741a5287f").AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		var apiResp *api.Compare
		DecodeJSON(t, resp, &apiResp)

		assert.Equal(t, 1, apiResp.TotalCommits)
		assert.Len(t, apiResp.Commits, 1)
	})
}
