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

	t.Run("Lock", func(t *testing.T) {
		issueBefore := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
		assert.False(t, issueBefore.IsLocked)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issueBefore.RepoID})
		owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/lock", owner.Name, repo.Name, issueBefore.Index)

		session := loginUser(t, owner.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

		// check lock issue
		req := NewRequestWithJSON(t, "PUT", urlStr, api.LockIssueOption{Reason: "Spam"}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)
		issueAfter := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
		assert.True(t, issueAfter.IsLocked)

		// check with other user
		user34 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 34})
		session34 := loginUser(t, user34.Name)
		token34 := getTokenForLoggedInUser(t, session34, auth_model.AccessTokenScopeAll)
		req = NewRequestWithJSON(t, "PUT", urlStr, api.LockIssueOption{Reason: "Spam"}).AddTokenAuth(token34)
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("Unlock", func(t *testing.T) {
		issueBefore := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issueBefore.RepoID})
		owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/lock", owner.Name, repo.Name, issueBefore.Index)

		session := loginUser(t, owner.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

		lockReq := NewRequestWithJSON(t, "PUT", urlStr, api.LockIssueOption{Reason: "Spam"}).AddTokenAuth(token)
		MakeRequest(t, lockReq, http.StatusNoContent)

		// check unlock issue
		req := NewRequest(t, "DELETE", urlStr).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)
		issueAfter := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
		assert.False(t, issueAfter.IsLocked)

		// check with other user
		user34 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 34})
		session34 := loginUser(t, user34.Name)
		token34 := getTokenForLoggedInUser(t, session34, auth_model.AccessTokenScopeAll)
		req = NewRequest(t, "DELETE", urlStr).AddTokenAuth(token34)
		MakeRequest(t, req, http.StatusForbidden)
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
