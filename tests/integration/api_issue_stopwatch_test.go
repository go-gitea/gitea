// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
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

func TestAPIListStopWatches(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopeReadUser)
	req := NewRequest(t, "GET", "/api/v1/user/stopwatches").
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	apiWatches := DecodeJSON(t, resp, []*api.StopWatch{})
	stopwatch := unittest.AssertExistsAndLoadBean(t, &issues_model.Stopwatch{UserID: owner.ID})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: stopwatch.IssueID})
	if assert.Len(t, apiWatches, 1) {
		assert.Equal(t, stopwatch.CreatedUnix.AsTime().Unix(), apiWatches[0].Created.Unix())
		assert.Equal(t, issue.Index, apiWatches[0].IssueIndex)
		assert.Equal(t, issue.Title, apiWatches[0].IssueTitle)
		assert.Equal(t, repo.Name, apiWatches[0].RepoName)
		assert.Equal(t, repo.OwnerName, apiWatches[0].RepoOwnerName)
		assert.Positive(t, apiWatches[0].Seconds)
	}

	for _, testCase := range []struct {
		name         string
		issueID      int64
		userID       int64
		method       string
		urlFormat    string
		expectedCode int
	}{
		{"stop", 2, 2, "POST", "/api/v1/repos/%s/%s/issues/%d/stopwatch/stop", http.StatusCreated},
		{"cancel", 1, 1, "DELETE", "/api/v1/repos/%s/%s/issues/%d/stopwatch/delete", http.StatusNoContent},
		{"start", 3, 2, "POST", "/api/v1/repos/%s/%s/issues/%d/stopwatch/start", http.StatusCreated},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.name == "start" {
				defer tests.PrepareTestEnv(t)()
			}

			issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: testCase.issueID})
			_ = issue.LoadRepo(t.Context())
			owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: issue.Repo.OwnerID})
			user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: testCase.userID})

			session := loginUser(t, user.Name)
			token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

			req := NewRequestf(t, testCase.method, testCase.urlFormat, owner.Name, issue.Repo.Name, issue.Index).
				AddTokenAuth(token)
			MakeRequest(t, req, testCase.expectedCode)
			MakeRequest(t, req, http.StatusConflict)
		})
	}
}
