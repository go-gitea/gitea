// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

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

func TestAPIDownloadCompareDiffOrPatch(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		session := loginUser(t, "user2")
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

		t.Run("BranchToBranchDiff", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", "/api/v1/repos/user2/repo20/compare/add-csv...remove-files-b?output=diff").AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusOK)
			assert.Equal(t, "text/plain; charset=utf-8", resp.Header().Get("Content-Type"))
			body := resp.Body.String()
			assert.Contains(t, body, "diff --git ")
		})

		t.Run("BranchToBranchPatch", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", "/api/v1/repos/user2/repo20/compare/add-csv...remove-files-b?output=patch").AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusOK)
			assert.Equal(t, "text/plain; charset=utf-8", resp.Header().Get("Content-Type"))
			body := resp.Body.String()
			assert.True(t, strings.HasPrefix(body, "From "), "patch output should start with a format-patch header, got: %q", body[:min(40, len(body))])
		})

		t.Run("CommitToCommitDiff", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", "/api/v1/repos/user2/repo20/compare/808038d2f71b0ab02099...c8e31bc7688741a5287f?output=diff").AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusOK)
			assert.Contains(t, resp.Body.String(), "diff --git ")
		})

		t.Run("BranchToCommitDiff", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			// 8babce96... is the head of remove-files-b; pairing it with add-csv guarantees a non-empty diff.
			req := NewRequest(t, "GET", "/api/v1/repos/user2/repo20/compare/add-csv...8babce967f21b9dfa6987f943b91093dac58a4f0?output=diff").AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusOK)
			assert.Contains(t, resp.Body.String(), "diff --git ")
		})

		t.Run("TwoDotSeparator", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", "/api/v1/repos/user2/repo20/compare/add-csv..remove-files-b?output=diff").AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusOK)
			assert.Contains(t, resp.Body.String(), "diff --git ")
		})

		t.Run("SlashedBranchName", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			// user2/repo1's `feature/1` branch contains a slash; the route must match it
			// without URL-encoding. master and feature/1 happen to share a SHA in the fixture,
			// so we only assert the route resolves (200 OK) rather than checking diff content.
			req := NewRequest(t, "GET", "/api/v1/repos/user2/repo1/compare/master...feature/1?output=diff").AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusOK)
			assert.Equal(t, "text/plain; charset=utf-8", resp.Header().Get("Content-Type"))
		})

		t.Run("UnknownOutputReturnsJSON", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			// Only "diff"/"patch" switch to raw output; any other value falls through to JSON.
			req := NewRequest(t, "GET", "/api/v1/repos/user2/repo20/compare/add-csv...remove-files-b?output=foo").AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusOK)
			apiResp := DecodeJSON(t, resp, &api.Compare{})
			assert.Equal(t, 2, apiResp.TotalCommits)
		})

		t.Run("SingleRefImplicitBase", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			// No `...`/`..` separator: parseCompareInfo defaults the base to the
			// repo's PR target branch (master for repo20) and compares it against
			// the given head.
			req := NewRequest(t, "GET", "/api/v1/repos/user2/repo20/compare/add-csv?output=diff").AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusOK)
			assert.Equal(t, "text/plain; charset=utf-8", resp.Header().Get("Content-Type"))
			assert.Contains(t, resp.Body.String(), "diff --git ")
		})

		t.Run("PrivateRepoAnonymous", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			// repo16 is private; an unauthenticated request must not leak its existence.
			req := NewRequest(t, "GET", "/api/v1/repos/user2/repo16/compare/master...good-sign?output=diff")
			MakeRequest(t, req, http.StatusNotFound)
		})

		t.Run("CrossRepoFork", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			user13 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 13})
			repo11 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 11})
			user13Sess := loginUser(t, "user13")
			user13Token := getTokenForLoggedInUser(t, user13Sess, auth_model.AccessTokenScopeWriteRepository)

			_, err := createFileInBranch(user13, repo11, createFileInBranchOptions{OldBranch: "master", NewBranch: "cross-repo-diff"}, map[string]string{"hello.txt": "hi\n"})
			require.NoError(t, err)

			req := NewRequest(t, "GET", "/api/v1/repos/user12/repo10/compare/master...user13:cross-repo-diff?output=diff").AddTokenAuth(user13Token)
			resp := MakeRequest(t, req, http.StatusOK)
			assert.Equal(t, "text/plain; charset=utf-8", resp.Header().Get("Content-Type"))
			assert.Contains(t, resp.Body.String(), "diff --git ")
		})
	})
}
