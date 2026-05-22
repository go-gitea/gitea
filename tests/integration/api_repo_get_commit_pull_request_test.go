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

	// Helper: query the /pulls endpoint and decode the response
	getCommitPRs := func(t *testing.T, sha string, expectedStatus int) []*api.PullRequest {
		t.Helper()
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/repo1/commits/%s/pulls", user.Name, sha).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, expectedStatus)

		var prs []*api.PullRequest
		DecodeJSON(t, resp, &prs)
		return prs
	}

	t.Run("MergedCommit", func(t *testing.T) {
		// PR #1 (fixture id=1) has merged_commit_id = 1a8823cd1a9549fde083f992f6b9b87a7ab74fb3
		// This tests the DB-level lookup by merged_commit_id
		mergedCommitSHA := "1a8823cd1a9549fde083f992f6b9b87a7ab74fb3"

		prs := getCommitPRs(t, mergedCommitSHA, http.StatusOK)

		assert.NotEmpty(t, prs, "Should find the PR by its merge commit SHA")
		assert.Equal(t, int64(2), prs[0].Index, "Should be PR index 2 (fixture PR #1)")
		assert.Equal(t, "master", prs[0].Base.Name)
	})

	t.Run("CommitInPRBranch", func(t *testing.T) {
		// Commit 5c050d3b is on branch2 (PR #2, fixture id=2) and pr-to-update (PR #5, fixture id=5)
		// This tests the git branch containment strategy
		commitOnBranch := "5c050d3b6d2db231ab1f64e324f1b6b9a0b181c2"

		prs := getCommitPRs(t, commitOnBranch, http.StatusOK)

		assert.NotEmpty(t, prs, "Should find PRs whose branches contain this commit")

		// Verify we found at least the PR with head_branch=branch2
		foundPR2 := false
		for _, pr := range prs {
			if pr.Index == 3 { // PR #2 has issue_id=3 so index=3
				foundPR2 = true
				assert.Equal(t, "branch2", pr.Head.Name)
			}
		}
		assert.True(t, foundPR2, "Expected to find PR with head_branch=branch2")
	})

	t.Run("InvalidCommitSHA", func(t *testing.T) {
		prs := getCommitPRs(t, "invalidsha", http.StatusOK)
		assert.Empty(t, prs, "Should return empty array for invalid SHA")
	})

	t.Run("NonexistentCommit", func(t *testing.T) {
		// Valid SHA format but doesn't exist in repo
		prs := getCommitPRs(t, "0000000000000000000000000000000000000000", http.StatusOK)
		assert.Empty(t, prs, "Should return empty array for nonexistent commit")
	})
}
