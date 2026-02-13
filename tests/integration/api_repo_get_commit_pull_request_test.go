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

	// Test with a commit that has an associated merged PR
	t.Run("ValidMergedCommit", func(t *testing.T) {
		// Use the actual merged commit SHA from your test fixtures
		mergedCommitSHA := "1a8823cd1a9549fde083f992f6b9b87a7ab74fb3"

		req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/commits/%s/pulls", user.Name, mergedCommitSHA).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		var pullRequests []*api.PullRequest
		DecodeJSON(t, resp, &pullRequests)

		assert.NotEmpty(t, pullRequests, "Should find at least one PR for this commit")
		// Verify the PR details match expectations
		assert.Equal(t, int64(2), pullRequests[0].Index)
		assert.Equal(t, "master", pullRequests[0].Base.Name)
	})

	t.Run("CommitWithNoPRs", func(t *testing.T) {
		// Use a valid commit that was never part of a PR — returns empty array
		commitWithoutPR := "65f1bf27bc3bf70f64657658635e66094edbcb4d"

		req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/commits/%s/pulls", user.Name, commitWithoutPR).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		var pullRequests []*api.PullRequest
		DecodeJSON(t, resp, &pullRequests)

		assert.Empty(t, pullRequests, "Should return empty array for commit without PRs")
	})

	t.Run("InvalidCommitSHA", func(t *testing.T) {
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/commits/%s/pulls", user.Name, "invalidsha").
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		var pullRequests []*api.PullRequest
		DecodeJSON(t, resp, &pullRequests)

		assert.Empty(t, pullRequests)
	})

	t.Run("NonexistentCommit", func(t *testing.T) {
		// Valid SHA format but doesn't exist in repo
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/commits/%s/pulls", user.Name, "0000000000000000000000000000000000000000").
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		var pullRequests []*api.PullRequest
		DecodeJSON(t, resp, &pullRequests)

		assert.Empty(t, pullRequests)
	})
}
