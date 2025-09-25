// Copyright 2020 The Gitea Authors. All rights reserved.
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

func TestAPIIssueSubscriptions(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	issue1 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	issue2 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	issue3 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 3})
	issue4 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 4})
	issue5 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 8})

	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: issue1.PosterID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

	testSubscription := func(issue *issues_model.Issue, isWatching bool) {
		issueRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/subscriptions/check", issueRepo.OwnerName, issueRepo.Name, issue.Index)).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		wi := new(api.WatchInfo)
		DecodeJSON(t, resp, wi)

		assert.Equal(t, isWatching, wi.Subscribed)
		assert.Equal(t, !isWatching, wi.Ignored)
		assert.Equal(t, issue.APIURL(t.Context())+"/subscriptions", wi.URL)
		assert.EqualValues(t, issue.CreatedUnix, wi.CreatedAt.Unix())
		assert.Equal(t, issueRepo.APIURL(), wi.RepositoryURL)
	}

	testSubscription(issue1, true)
	testSubscription(issue2, true)
	testSubscription(issue3, true)
	testSubscription(issue4, false)
	testSubscription(issue5, false)

	issue1Repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue1.RepoID})
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/subscriptions/%s", issue1Repo.OwnerName, issue1Repo.Name, issue1.Index, owner.Name)
	req := NewRequest(t, "DELETE", urlStr).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)
	testSubscription(issue1, false)

	req = NewRequest(t, "DELETE", urlStr).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)
	testSubscription(issue1, false)

	issue5Repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue5.RepoID})
	urlStr = fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/subscriptions/%s", issue5Repo.OwnerName, issue5Repo.Name, issue5.Index, owner.Name)
	req = NewRequest(t, "PUT", urlStr).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)
	testSubscription(issue5, true)

	req = NewRequest(t, "PUT", urlStr).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)
	testSubscription(issue5, true)
}
