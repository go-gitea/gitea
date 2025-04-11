// Copyright 2025 The Gitea Authors. All rights reserved.
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
)

func TestAPILockIssue(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	issueBefore := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	assert.False(t, issueBefore.IsLocked)
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issueBefore.RepoID})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/lock", owner.Name, repo.Name, issueBefore.Index)

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

	// check invalid reason
	req := NewRequestWithJSON(t, "PUT", urlStr, api.LockIssueOption{Reason: "Not valid"}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusBadRequest)

	// check lock issue
	req = NewRequestWithJSON(t, "PUT", urlStr, api.LockIssueOption{Reason: "Spam"}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
	issueAfter := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	assert.True(t, issueAfter.IsLocked)

	// check reason is case insensitive
	unlockReq := NewRequest(t, "DELETE", urlStr).AddTokenAuth(token)
	MakeRequest(t, unlockReq, http.StatusNoContent)
	issueAfter = unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	assert.False(t, issueAfter.IsLocked)

	req = NewRequestWithJSON(t, "PUT", urlStr, api.LockIssueOption{Reason: "too heated"}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
	issueAfter = unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	assert.True(t, issueAfter.IsLocked)

	// check with other user
	user34 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 34})
	session34 := loginUser(t, user34.Name)
	token34 := getTokenForLoggedInUser(t, session34, auth_model.AccessTokenScopeAll)
	req = NewRequestWithJSON(t, "PUT", urlStr, api.LockIssueOption{Reason: "Spam"}).AddTokenAuth(token34)
	MakeRequest(t, req, http.StatusForbidden)
}

func TestAPIUnlockIssue(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

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
}
