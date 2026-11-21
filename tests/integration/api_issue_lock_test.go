// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPILockIssue(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	urlStr := "/api/v1/repos/user2/repo1/issues/1/lock"
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteIssue)
	token34 := getUserToken(t, unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 34}).Name, auth_model.AccessTokenScopeAll)

	t.Run("Lock", func(t *testing.T) {
		assert.False(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1}).IsLocked)
		MakeRequest(t, NewRequestWithJSON(t, "PUT", urlStr, api.LockIssueOption{}).AddTokenAuth(token), http.StatusNoContent)
		assert.True(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1}).IsLocked)
		MakeRequest(t, NewRequestWithJSON(t, "PUT", urlStr, api.LockIssueOption{}).AddTokenAuth(token34), http.StatusForbidden)
	})

	t.Run("AuthorCanStopStopwatchButNotEditLockedIssue", func(t *testing.T) {
		session := loginUser(t, "user5")
		issueURL := testNewIssue(t, session, "user2", "repo1", "title", "content")
		session.MakeRequest(t, NewRequest(t, "POST", issueURL+"/times/stopwatch/start"), http.StatusOK)
		apiURL := "/api/v1/repos" + issueURL
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
		MakeRequest(t, NewRequestWithJSON(t, "PUT", urlStr, api.LockIssueOption{}).AddTokenAuth(token), http.StatusNoContent)
		MakeRequest(t, NewRequest(t, "DELETE", urlStr).AddTokenAuth(token), http.StatusNoContent)
		assert.False(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1}).IsLocked)
		MakeRequest(t, NewRequest(t, "DELETE", urlStr).AddTokenAuth(token34), http.StatusForbidden)
	})

	t.Run("DoerOnOrganizationRepo", func(t *testing.T) {
		// the lock and unlock comments are posted by the signed-in user, not by the organization owning the repository
		issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 6})
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})
		org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
		require.True(t, org.IsOrganization())
		doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2"}) // an owner of the organization
		session := loginUser(t, doer.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue, auth_model.AccessTokenScopeWriteRepository)
		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/lock", org.Name, repo.Name, issue.Index)

		req := NewRequestWithJSON(t, "PUT", urlStr, api.LockIssueOption{Reason: "Spam"}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)
		lockComment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: issue.ID, Type: issues_model.CommentTypeLock})
		assert.Equal(t, doer.ID, lockComment.PosterID)

		req = NewRequest(t, "DELETE", urlStr).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)
		unlockComment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: issue.ID, Type: issues_model.CommentTypeUnlock})
		assert.Equal(t, doer.ID, unlockComment.PosterID)
	})
}
