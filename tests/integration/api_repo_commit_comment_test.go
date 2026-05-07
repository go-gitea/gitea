// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// commitFixtureSHA is a known full SHA in the user2/repo1 fixture, taken from
// existing tests in api_repo_git_commits_test.go. Using a fixture SHA keeps
// this test hermetic: we don't need to materialize new git history.
const commitFixtureSHA = "65f1bf27bc3bf70f64657658635e66094edbcb4d"

func TestAPICommitComments(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	t.Run("ListEmpty", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/commits/%s/comments",
			repoOwner.Name, repo.Name, commitFixtureSHA)
		resp := MakeRequest(t, req, http.StatusOK)
		comments := DecodeJSON(t, resp, []*api.Comment{})
		assert.Empty(t, comments, "no comments should exist before any are posted")
	})

	token := getUserToken(t, repoOwner.Name, auth_model.AccessTokenScopeWriteRepository)

	t.Run("CreateGeneral", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		body := "general thoughts on this commit"
		req := NewRequestWithJSON(t, "POST",
			fmt.Sprintf("/api/v1/repos/%s/%s/commits/%s/comments",
				repoOwner.Name, repo.Name, commitFixtureSHA),
			&api.CreateCommitCommentOption{Body: body},
		).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusCreated)

		created := DecodeJSON(t, resp, &api.Comment{})
		assert.Equal(t, body, created.Body)
		assert.NotZero(t, created.ID)

		// And confirm it persisted as a CommentTypeComment hung off a synthetic
		// carrier Issue (CommitSHA set, IsPull false).
		dbComment := unittest.AssertExistsAndLoadBean(t,
			&issues_model.Comment{ID: created.ID, Type: issues_model.CommentTypeComment, Content: body})
		issue := unittest.AssertExistsAndLoadBean(t,
			&issues_model.Issue{ID: dbComment.IssueID})
		assert.Equal(t, commitFixtureSHA, issue.CommitSHA)
		assert.False(t, issue.IsPull)
	})

	t.Run("CreateInline", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		body := "this line could be clearer"
		req := NewRequestWithJSON(t, "POST",
			fmt.Sprintf("/api/v1/repos/%s/%s/commits/%s/comments",
				repoOwner.Name, repo.Name, commitFixtureSHA),
			&api.CreateCommitCommentOption{
				Body: body,
				Path: "README.md",
				Line: 7,
			},
		).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusCreated)

		created := DecodeJSON(t, resp, &api.Comment{})
		assert.Equal(t, body, created.Body)

		// Inline comments are stored as CommentTypeCode with line + path set.
		unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{
			ID:       created.ID,
			Type:     issues_model.CommentTypeCode,
			TreePath: "README.md",
			Line:     7,
			Content:  body,
		})
	})

	t.Run("CarrierReused", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		// Both CreateGeneral and CreateInline targeted the same SHA; the
		// synthetic carrier Issue must be a single row, not one per comment.
		issues := make([]*issues_model.Issue, 0)
		err := unittest.GetXORMEngine().
			Where("repo_id = ? AND commit_sha = ?", repo.ID, commitFixtureSHA).
			Find(&issues)
		require.NoError(t, err)
		assert.Len(t, issues, 1, "exactly one carrier Issue per (repo, sha)")
	})

	t.Run("ListPopulated", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/commits/%s/comments",
			repoOwner.Name, repo.Name, commitFixtureSHA)
		resp := MakeRequest(t, req, http.StatusOK)
		comments := DecodeJSON(t, resp, []*api.Comment{})
		assert.Len(t, comments, 2, "general + inline comment")
	})

	t.Run("InvalidSHAReturns404", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequestWithJSON(t, "POST",
			fmt.Sprintf("/api/v1/repos/%s/%s/commits/%s/comments",
				repoOwner.Name, repo.Name, "0000000000000000000000000000000000000000"),
			&api.CreateCommitCommentOption{Body: "should not happen"},
		).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("UnauthenticatedCreateForbidden", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequestWithJSON(t, "POST",
			fmt.Sprintf("/api/v1/repos/%s/%s/commits/%s/comments",
				repoOwner.Name, repo.Name, commitFixtureSHA),
			&api.CreateCommitCommentOption{Body: "no token"},
		)
		MakeRequest(t, req, http.StatusUnauthorized)
	})

	t.Run("CarrierIssueHiddenFromIssuesAPI", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		// The synthetic carrier Issue must NOT show up in the regular issue
		// listings — it isn't a real issue, just a comment carrier. This is
		// the property that lets the rest of the codebase keep treating
		// `Issue` rows uniformly without ever caring about commit comments.
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/issues?type=issues&state=all",
			repoOwner.Name, repo.Name).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var issues []*api.Issue
		DecodeJSON(t, resp, &issues)
		for _, iss := range issues {
			assert.NotContains(t, iss.Title, "Comments on commit "+commitFixtureSHA,
				"carrier Issue must not leak into the user-facing issue list")
		}
	})
}
