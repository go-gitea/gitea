// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPICompareBranches(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		session2 := loginUser(t, "user2")
		token2 := getTokenForLoggedInUser(t, session2, auth_model.AccessTokenScopeWriteRepository)

		t.Run("CompareBranches", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequestf(t, "GET", "/api/v1/repos/user2/repo20/compare/add-csv...remove-files-b").AddTokenAuth(token2)
			resp := MakeRequest(t, req, http.StatusOK)
			apiResp := DecodeJSON(t, resp, &api.Compare{})
			assert.Equal(t, 2, apiResp.TotalCommits)
			assert.Len(t, apiResp.Commits, 2)
		})

		t.Run("CompareCommits", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequestf(t, "GET", "/api/v1/repos/user2/repo20/compare/808038d2f71b0ab02099...c8e31bc7688741a5287f").AddTokenAuth(token2)
			resp := MakeRequest(t, req, http.StatusOK)
			apiResp := DecodeJSON(t, resp, &api.Compare{})
			assert.Equal(t, 1, apiResp.TotalCommits)
			assert.Len(t, apiResp.Commits, 1)
		})

		t.Run("CompareForkOnlyCommit", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			user13 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 13})
			repo11 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 11})
			user13Sess := loginUser(t, "user13")
			user13Token := getTokenForLoggedInUser(t, user13Sess, auth_model.AccessTokenScopeWriteRepository)

			_, err := createFileInBranch(user13, repo11, createFileInBranchOptions{OldBranch: "master", NewBranch: "new-branch"}, map[string]string{"file.txt": "content"})
			require.NoError(t, err)
			req := NewRequestf(t, "GET", "/api/v1/repos/user12/repo10/compare/master...user13:new-branch").AddTokenAuth(user13Token)
			resp := MakeRequest(t, req, http.StatusOK)
			apiResp := DecodeJSON(t, resp, &api.Compare{})
			assert.Equal(t, 1, apiResp.TotalCommits)
			assert.Len(t, apiResp.Commits, 1)
		})
	})
}
