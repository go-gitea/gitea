// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"path"
	"testing"

	auth_model "gitea.dev/models/auth"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPILockIssue(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	issueBefore := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issueBefore.RepoID})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/lock", owner.Name, repo.Name, issueBefore.Index)
	token := getUserToken(t, owner.Name, auth_model.AccessTokenScopeWriteIssue)
	token34 := getUserToken(t, unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 34}).Name, auth_model.AccessTokenScopeAll)

	t.Run("Lock", func(t *testing.T) {
		assert.False(t, issueBefore.IsLocked)
		MakeRequest(t, NewRequestWithJSON(t, "PUT", urlStr, api.LockIssueOption{Reason: "Spam"}).AddTokenAuth(token), http.StatusNoContent)
		assert.True(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1}).IsLocked)
		MakeRequest(t, NewRequestWithJSON(t, "PUT", urlStr, api.LockIssueOption{Reason: "Spam"}).AddTokenAuth(token34), http.StatusForbidden)
	})

	t.Run("AuthorCanStopStopwatchButNotEditLockedIssue", func(t *testing.T) {
		session := loginUser(t, "user5")
		issueURL := testNewIssue(t, session, "user2", "repo1", "title", "content")
		session.MakeRequest(t, NewRequest(t, "POST", issueURL+"/times/stopwatch/start"), http.StatusOK)
		apiURL := "/api/v1/repos/user2/repo1/issues/" + path.Base(issueURL)
		MakeRequest(t, NewRequestWithJSON(t, "PUT", apiURL+"/lock", api.LockIssueOption{}).AddTokenAuth(token), http.StatusNoContent)
		MakeRequest(t, NewRequestWithJSON(t, "PATCH", apiURL, api.EditIssueOption{Title: "edited"}).AddTokenAuth(getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)), http.StatusForbidden)
		AssertHTMLElement(t, NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", issueURL), http.StatusOK).Body), "#issue-title-edit-show", false)
		session.MakeRequest(t, NewRequest(t, "POST", issueURL+"/times/stopwatch/stop"), http.StatusOK)
	})

	t.Run("AuthorCanChangeAllowMaintainerEditOfLockedPull", func(t *testing.T) {
		assert.NoError(t, issues_model.UpdateIssueCols(t.Context(), &issues_model.Issue{ID: 8, PosterID: 13}, "poster_id"))
		MakeRequest(t, NewRequestWithJSON(t, "PUT", "/api/v1/repos/user12/repo10/issues/1/lock", api.LockIssueOption{}).AddTokenAuth(getUserToken(t, "user12", auth_model.AccessTokenScopeWriteIssue)), http.StatusNoContent)
		MakeRequest(t, NewRequestWithJSON(t, "PATCH", "/api/v1/repos/user12/repo10/pulls/1", &api.EditPullRequestOption{AllowMaintainerEdit: new(false)}).AddTokenAuth(getUserToken(t, "user13", auth_model.AccessTokenScopeWriteRepository)), http.StatusCreated)
	})

	t.Run("Unlock", func(t *testing.T) {
		MakeRequest(t, NewRequestWithJSON(t, "PUT", urlStr, api.LockIssueOption{Reason: "Spam"}).AddTokenAuth(token), http.StatusNoContent)
		MakeRequest(t, NewRequest(t, "DELETE", urlStr).AddTokenAuth(token), http.StatusNoContent)
		assert.False(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1}).IsLocked)
		MakeRequest(t, NewRequest(t, "DELETE", urlStr).AddTokenAuth(token34), http.StatusForbidden)
	})
}
