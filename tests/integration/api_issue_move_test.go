// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIMoveCopyIssue(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue, auth_model.AccessTokenScopeWriteRepository)

	t.Run("NoPermission", func(t *testing.T) {
		token5 := getTokenForLoggedInUser(t, loginUser(t, "user5"), auth_model.AccessTokenScopeAll)
		req := NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/issues/1/move", api.MoveIssueOption{TargetRepo: "user2/repo2"}).AddTokenAuth(token5)
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("InvalidTarget", func(t *testing.T) {
		req := NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/issues/1/move", api.MoveIssueOption{TargetRepo: "user2/not-exist"}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusUnprocessableEntity)
		req = NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/issues/1/move", api.MoveIssueOption{TargetRepo: "user2/repo1"}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusBadRequest)
	})

	t.Run("Move", func(t *testing.T) {
		req := NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/issues/1/move", api.MoveIssueOption{TargetRepo: "user2/repo2", KeepLabels: true}).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		apiIssue := DecodeJSON(t, resp, &api.Issue{})
		assert.Equal(t, int64(1), apiIssue.ID)
		assert.Equal(t, "user2/repo2", apiIssue.Repo.FullName)
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1, RepoID: 2, Index: apiIssue.Index})

		// the old link redirects to the new location
		req = NewRequest(t, "GET", "/user2/repo1/issues/1")
		resp = session.MakeRequest(t, req, http.StatusSeeOther)
		assert.Equal(t, fmt.Sprintf("/user2/repo2/issues/%d", apiIssue.Index), resp.Header().Get("Location"))
		// but it doesn't reveal the private target repository to users who can't read it
		MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/issues/1"), http.StatusNotFound)
	})

	t.Run("Copy", func(t *testing.T) {
		req := NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/issues/4/copy", api.CopyIssueOption{TargetRepo: "user2/repo2"}).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusCreated)
		apiIssue := DecodeJSON(t, resp, &api.Issue{})
		original := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: 1, Index: 4})
		copied := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: apiIssue.ID, RepoID: 2})
		assert.NotEqual(t, original.ID, copied.ID)
		assert.Equal(t, original.Title, copied.Title)
		assert.False(t, copied.IsClosed)
	})

	t.Run("WebMove", func(t *testing.T) {
		req := NewRequest(t, "GET", "/user2/repo1/issues/4")
		resp := session.MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, 1, NewHTMLParser(t, resp.Body).Find("#sidebar-move-copy-issue form[action='/user2/repo1/issues/4/move']").Length())

		req = NewRequestWithValues(t, "GET", "/user2/repo1/issues/move-targets", map[string]string{"q": "repo2"})
		resp = session.MakeRequest(t, req, http.StatusOK)
		assert.Contains(t, resp.Body.String(), `"full_name":"user2/repo2"`)

		req = NewRequestWithValues(t, "POST", "/user2/repo1/issues/4/move", map[string]string{"target_repo": "user2/repo2", "keep_milestone": "true"})
		resp = session.MakeRequest(t, req, http.StatusOK)
		moved := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 5})
		assert.EqualValues(t, 2, moved.RepoID)
		assert.Equal(t, fmt.Sprintf("/user2/repo2/issues/%d", moved.Index), test.RedirectURL(resp))
	})
}
