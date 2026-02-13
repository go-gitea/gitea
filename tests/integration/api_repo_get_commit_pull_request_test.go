// Copyright 2026 The Gitea Authors. All rights reserved.
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

func TestAPIReposGetCommitPullRequests(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	// PR #1 in repo1 was merged with commit 1a8823cd1a9549fde083f992f6b9b87a7ab74fb3
	mergedCommitSHA := "1a8823cd1a9549fde083f992f6b9b87a7ab74fb3"

	t.Run("ValidMergedCommit", func(t *testing.T) {
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/commits/%s/pulls", user.Name, mergedCommitSHA).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		var pullRequests []*api.PullRequest
		DecodeJSON(t, resp, &pullRequests)

		assert.NotEmpty(t, pullRequests)
		assert.Equal(t, int64(2), pullRequests[0].Index)
	})

	t.Run("NoAssociatedPR", func(t *testing.T) {
		// A commit that was not part of any merged PR
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/commits/%s/pulls", user.Name, "0000000000000000000000000000000000000000").
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})
}
