// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func getEventWatchers(t *testing.T, repo *repo_model.Repository, event string) []api.User {
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/subscribers/%s", repo.OwnerName, repo.Name, event)

	req := NewRequest(t, "GET", urlStr)
	resp := MakeRequest(t, req, http.StatusOK)

	var watchers []api.User
	DecodeJSON(t, resp, &watchers)

	return watchers
}

func addIssueComment(t *testing.T, repo *repo_model.Repository, issueIndex int, token, text string) {
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/comments?token=%s",
		repo.OwnerName, repo.Name, issueIndex, token)
	req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
		"body": text,
	})
	MakeRequest(t, req, http.StatusCreated)
}

func getNotifications(t *testing.T, token string) []api.NotificationThread {
	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/notifications?token=%s", token))
	resp := MakeRequest(t, req, http.StatusOK)

	var notifications []api.NotificationThread
	DecodeJSON(t, resp, &notifications)

	return notifications
}

func TestAPIRepoCustomWatch(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	session := loginUser(t, repo.OwnerName)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteNotification)

	session2 := loginUser(t, "user12")
	token2 := getTokenForLoggedInUser(t, session2, auth_model.AccessTokenScopeWriteIssue)

	// make sure nobdy is watching the test repo
	_, err := db.GetEngine(db.DefaultContext).Exec("DELETE FROM watch WHERE repo_id = ?", repo.ID)
	assert.NoError(t, err)

	assert.Len(t, getEventWatchers(t, repo, "issues"), 0)
	assert.Len(t, getEventWatchers(t, repo, "pullrequests"), 0)
	assert.Len(t, getEventWatchers(t, repo, "releases"), 0)

	const IssueIndex = 1
	const PullRequestIndex = 5

	t.Run("Issues", func(t *testing.T) {
		// Make sure there are no unread notifications
		MakeRequest(t, NewRequest(t, "PUT", fmt.Sprintf("/api/v1/notifications?status-types=unread&status-types=pinned&token=%s", token)), http.StatusResetContent)

		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/subscription/custom?token=%s", repo.OwnerName, repo.Name, token)
		req := NewRequestWithJSON(t, "PUT", urlStr, api.RepoCustomWatchOptions{Issues: true})
		MakeRequest(t, req, http.StatusOK)

		assert.Len(t, getEventWatchers(t, repo, "pullrequests"), 0)
		assert.Len(t, getEventWatchers(t, repo, "releases"), 0)

		watchers := getEventWatchers(t, repo, "issues")
		assert.Len(t, watchers, 1)
		assert.Equal(t, repo.OwnerID, watchers[0].ID)

		addIssueComment(t, repo, IssueIndex, token2, "Hello Issue")
		addIssueComment(t, repo, PullRequestIndex, token2, "Hello PR")

		notifications := getNotifications(t, token)
		assert.Len(t, notifications, 1)
		assert.Equal(t, "issue1", notifications[0].Subject.Title)
	})

	t.Run("PullRequests", func(t *testing.T) {
		// Make sure there are no unread notifications
		MakeRequest(t, NewRequest(t, "PUT", fmt.Sprintf("/api/v1/notifications?status-types=unread&status-types=pinned&token=%s", token)), http.StatusResetContent)

		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/subscription/custom?token=%s", repo.OwnerName, repo.Name, token)
		req := NewRequestWithJSON(t, "PUT", urlStr, api.RepoCustomWatchOptions{PullRequests: true})
		MakeRequest(t, req, http.StatusOK)

		assert.Len(t, getEventWatchers(t, repo, "issues"), 0)
		assert.Len(t, getEventWatchers(t, repo, "releases"), 0)

		watchers := getEventWatchers(t, repo, "pullrequests")
		assert.Len(t, watchers, 1)
		assert.Equal(t, repo.OwnerID, watchers[0].ID)

		addIssueComment(t, repo, IssueIndex, token2, "Hello Issue")
		addIssueComment(t, repo, PullRequestIndex, token2, "Hello PR")

		notifications := getNotifications(t, token)
		assert.Len(t, notifications, 1)
		assert.Equal(t, "pull5", notifications[0].Subject.Title)
	})

	t.Run("IssuesAndPullRequests", func(t *testing.T) {
		// Make sure there are no unread notifications
		MakeRequest(t, NewRequest(t, "PUT", fmt.Sprintf("/api/v1/notifications?status-types=unread&status-types=pinned&token=%s", token)), http.StatusResetContent)

		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/subscription/custom?token=%s", repo.OwnerName, repo.Name, token)
		req := NewRequestWithJSON(t, "PUT", urlStr, api.RepoCustomWatchOptions{Issues: true, PullRequests: true})
		MakeRequest(t, req, http.StatusOK)

		assert.Len(t, getEventWatchers(t, repo, "releases"), 0)

		issueWatchers := getEventWatchers(t, repo, "issues")
		assert.Len(t, issueWatchers, 1)
		assert.Equal(t, repo.OwnerID, issueWatchers[0].ID)

		pullRequestWatchers := getEventWatchers(t, repo, "pullrequests")
		assert.Len(t, pullRequestWatchers, 1)
		assert.Equal(t, repo.OwnerID, pullRequestWatchers[0].ID)

		addIssueComment(t, repo, IssueIndex, token2, "Hello Issue")
		addIssueComment(t, repo, PullRequestIndex, token2, "Hello PR")

		notifications := getNotifications(t, token)
		assert.Len(t, notifications, 2)

		// The notifications are not always in the same order
		if notifications[0].Subject.Title == "issue1" {
			assert.Equal(t, "issue1", notifications[0].Subject.Title)
			assert.Equal(t, "pull5", notifications[1].Subject.Title)
		} else {
			assert.Equal(t, "pull5", notifications[0].Subject.Title)
			assert.Equal(t, "issue1", notifications[1].Subject.Title)
		}
	})

	t.Run("Releases", func(t *testing.T) {
		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/subscription/custom?token=%s", repo.OwnerName, repo.Name, token)
		req := NewRequestWithJSON(t, "PUT", urlStr, api.RepoCustomWatchOptions{Releases: true})
		MakeRequest(t, req, http.StatusOK)

		assert.Len(t, getEventWatchers(t, repo, "issues"), 0)
		assert.Len(t, getEventWatchers(t, repo, "pullrequests"), 0)

		watchers := getEventWatchers(t, repo, "releases")
		assert.Len(t, watchers, 1)
		assert.Equal(t, repo.OwnerID, watchers[0].ID)

		// Release notifications are currently not shown in the UI, so we can't test that yet
	})
}

func TestAPIRepoCustomWatchInvalidEventType(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/subscribers/invalid", repo.OwnerName, repo.Name)), http.StatusBadRequest)
}
