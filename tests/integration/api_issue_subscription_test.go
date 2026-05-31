// Copyright 2020 The Gitea Authors. All rights reserved.
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
)

func TestAPIIssueSubscriptions(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	issue1 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	issue2 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	issue3 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 3})
	issue4 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 4})
	issue5 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 8})
	issue7 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 7})

	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: issue1.PosterID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

	testSubscription := func(issue *issues_model.Issue, isWatching bool) {
		issueRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/subscriptions/check", issueRepo.OwnerName, issueRepo.Name, issue.Index)).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		wi := DecodeJSON(t, resp, &api.WatchInfo{})

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

	testSubscribers := func(issue *issues_model.Issue, expectedNames []string, expectedTotal string) {
		issueRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/subscriptions", issueRepo.OwnerName, issueRepo.Name, issue.Index)).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		users := DecodeJSON(t, resp, []*api.User{})
		names := make([]string, 0, len(users))
		for _, user := range users {
			names = append(names, user.UserName)
		}
		assert.ElementsMatch(t, expectedNames, names)
		assert.Equal(t, expectedTotal, resp.Header().Get("X-Total-Count"))
	}

	testSubscribers(issue1, []string{"user1", "user4", "user5", "user11"}, "4")
	testSubscribers(issue7, []string{"user2"}, "1")

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
