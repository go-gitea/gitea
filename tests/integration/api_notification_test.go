// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	activities_model "code.gitea.io/gitea/models/activities"
	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	repo_service "code.gitea.io/gitea/services/repository"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPINotification(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	thread5 := unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{ID: 5})
	assert.NoError(t, thread5.LoadAttributes(t.Context()))
	session := loginUser(t, user2.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteNotification, auth_model.AccessTokenScopeWriteRepository)

	MakeRequest(t, NewRequest(t, "GET", "/api/v1/notifications"), http.StatusUnauthorized)

	// -- GET /notifications --
	// test filter
	since := "2000-01-01T00%3A50%3A01%2B00%3A00" // 946687801
	req := NewRequest(t, "GET", "/api/v1/notifications?since="+since).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	var apiNL []api.NotificationThread
	DecodeJSON(t, resp, &apiNL)

	assert.Len(t, apiNL, 1)
	assert.EqualValues(t, 5, apiNL[0].ID)

	// test filter
	before := "2000-01-01T01%3A06%3A59%2B00%3A00" // 946688819

	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/notifications?all=%s&before=%s", "true", before)).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiNL)

	assert.Len(t, apiNL, 3)
	assert.EqualValues(t, 4, apiNL[0].ID)
	assert.True(t, apiNL[0].Unread)
	assert.False(t, apiNL[0].Pinned)
	assert.EqualValues(t, 3, apiNL[1].ID)
	assert.False(t, apiNL[1].Unread)
	assert.True(t, apiNL[1].Pinned)
	assert.EqualValues(t, 2, apiNL[2].ID)
	assert.False(t, apiNL[2].Unread)
	assert.False(t, apiNL[2].Pinned)

	// -- GET /repos/{owner}/{repo}/notifications --
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/notifications?status-types=unread", user2.Name, repo1.Name)).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiNL)

	assert.Len(t, apiNL, 1)
	assert.EqualValues(t, 4, apiNL[0].ID)

	// -- GET /repos/{owner}/{repo}/notifications -- multiple status-types
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/notifications?status-types=unread&status-types=pinned", user2.Name, repo1.Name)).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiNL)

	assert.Len(t, apiNL, 2)
	assert.EqualValues(t, 4, apiNL[0].ID)
	assert.True(t, apiNL[0].Unread)
	assert.False(t, apiNL[0].Pinned)
	assert.EqualValues(t, 3, apiNL[1].ID)
	assert.False(t, apiNL[1].Unread)
	assert.True(t, apiNL[1].Pinned)

	MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("/api/v1/notifications/threads/%d", 1)), http.StatusUnauthorized)

	// -- GET /notifications/threads/{id} --
	// get forbidden
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/notifications/threads/%d", 1)).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusForbidden)

	// get own
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/notifications/threads/%d", thread5.ID)).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	var apiN api.NotificationThread
	DecodeJSON(t, resp, &apiN)

	assert.EqualValues(t, 5, apiN.ID)
	assert.False(t, apiN.Pinned)
	assert.True(t, apiN.Unread)
	assert.Equal(t, "issue4", apiN.Subject.Title)
	assert.EqualValues(t, "Issue", apiN.Subject.Type)
	assert.Equal(t, thread5.Issue.APIURL(t.Context()), apiN.Subject.URL)
	assert.Equal(t, thread5.Repository.HTMLURL(), apiN.Repository.HTMLURL)

	MakeRequest(t, NewRequest(t, "GET", "/api/v1/notifications/new"), http.StatusUnauthorized)

	newStruct := struct {
		New int64 `json:"new"`
	}{}

	// -- check notifications --
	req = NewRequest(t, "GET", "/api/v1/notifications/new").
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &newStruct)
	assert.Positive(t, newStruct.New)

	// -- mark notifications as read --
	req = NewRequest(t, "GET", "/api/v1/notifications?status-types=unread").
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiNL)
	assert.Len(t, apiNL, 2)

	lastReadAt := "2000-01-01T00%3A50%3A01%2B00%3A00" // 946687801 <- only Notification 4 is in this filter ...
	req = NewRequest(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/%s/notifications?last_read_at=%s", user2.Name, repo1.Name, lastReadAt)).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusResetContent)

	req = NewRequest(t, "GET", "/api/v1/notifications?status-types=unread").
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiNL)
	assert.Len(t, apiNL, 1)

	// -- PATCH /notifications/threads/{id} --
	req = NewRequest(t, "PATCH", fmt.Sprintf("/api/v1/notifications/threads/%d", thread5.ID)).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusResetContent)

	assert.Equal(t, activities_model.NotificationStatusUnread, thread5.Status)
	thread5 = unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{ID: 5})
	assert.Equal(t, activities_model.NotificationStatusRead, thread5.Status)

	// -- check notifications --
	req = NewRequest(t, "GET", "/api/v1/notifications/new").
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &newStruct)
	assert.Zero(t, newStruct.New)
}

func TestAPINotificationPUT(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	thread5 := unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{ID: 5})
	assert.NoError(t, thread5.LoadAttributes(t.Context()))
	session := loginUser(t, user2.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteNotification)

	// Check notifications are as expected
	req := NewRequest(t, "GET", "/api/v1/notifications?all=true").
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	var apiNL []api.NotificationThread
	DecodeJSON(t, resp, &apiNL)

	assert.Len(t, apiNL, 4)
	assert.EqualValues(t, 5, apiNL[0].ID)
	assert.True(t, apiNL[0].Unread)
	assert.False(t, apiNL[0].Pinned)
	assert.EqualValues(t, 4, apiNL[1].ID)
	assert.True(t, apiNL[1].Unread)
	assert.False(t, apiNL[1].Pinned)
	assert.EqualValues(t, 3, apiNL[2].ID)
	assert.False(t, apiNL[2].Unread)
	assert.True(t, apiNL[2].Pinned)
	assert.EqualValues(t, 2, apiNL[3].ID)
	assert.False(t, apiNL[3].Unread)
	assert.False(t, apiNL[3].Pinned)

	//
	// Notification ID 2 is the only one with status-type read & pinned
	// change it to unread.
	//
	req = NewRequest(t, "PUT", "/api/v1/notifications?status-types=read&status-type=pinned&to-status=unread").
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusResetContent)
	DecodeJSON(t, resp, &apiNL)
	assert.Len(t, apiNL, 1)
	assert.EqualValues(t, 2, apiNL[0].ID)
	assert.True(t, apiNL[0].Unread)
	assert.False(t, apiNL[0].Pinned)

	//
	// Now nofication ID 2 is the first in the list and is unread.
	//
	req = NewRequest(t, "GET", "/api/v1/notifications?all=true").
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiNL)

	assert.Len(t, apiNL, 4)
	assert.EqualValues(t, 2, apiNL[0].ID)
	assert.True(t, apiNL[0].Unread)
	assert.False(t, apiNL[0].Pinned)
}

func TestAPICommitNotification(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

		session := loginUser(t, user2.Name)
		token1 := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		content := "This is a test commit"
		contentEncoded := base64.StdEncoding.EncodeToString([]byte(content))
		// push a commit with @user2 in the commit message, it's expected to create a notification
		createFileOptions := api.CreateFileOptions{
			FileOptions: api.FileOptions{
				BranchName:    "master",
				NewBranchName: "master",
				Message:       "This is a test commit to mention @user2",
				Author: api.Identity{
					Name:  "Anne Doe",
					Email: "annedoe@example.com",
				},
				Committer: api.Identity{
					Name:  "John Doe",
					Email: "johndoe@example.com",
				},
				Dates: api.CommitDateOptions{
					Author:    time.Unix(946684810, 0),
					Committer: time.Unix(978307190, 0),
				},
			},
			ContentBase64: contentEncoded,
		}

		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/contents/new_commit_notification.txt", user2.Name, repo1.Name), &createFileOptions).
			AddTokenAuth(token1)
		MakeRequest(t, req, http.StatusCreated)

		// Check notifications are as expected
		token2 := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteNotification)
		req = NewRequest(t, "GET", "/api/v1/notifications?all=true").
			AddTokenAuth(token2)
		resp := MakeRequest(t, req, http.StatusOK)
		var apiNL []api.NotificationThread
		DecodeJSON(t, resp, &apiNL)

		assert.Equal(t, api.NotifySubjectCommit, apiNL[0].Subject.Type)
		assert.Equal(t, "This is a test commit to mention @user2", apiNL[0].Subject.Title)
		assert.True(t, apiNL[0].Unread)
		assert.False(t, apiNL[0].Pinned)
	})
}

func TestAPIReleaseNotification(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

		session1 := loginUser(t, user1.Name)
		token1 := getTokenForLoggedInUser(t, session1, auth_model.AccessTokenScopeWriteRepository)

		// user1 create a release, it's expected to create a notification
		createNewReleaseUsingAPI(t, token1, user2, repo1, "v0.0.2", "", "v0.0.2 is released", "test notification release")

		// user2 login to check notifications
		session2 := loginUser(t, user2.Name)

		// Check notifications are as expected
		token2 := getTokenForLoggedInUser(t, session2, auth_model.AccessTokenScopeWriteNotification)
		req := NewRequest(t, "GET", "/api/v1/notifications?all=true").
			AddTokenAuth(token2)
		resp := MakeRequest(t, req, http.StatusOK)
		var apiNL []api.NotificationThread
		DecodeJSON(t, resp, &apiNL)

		assert.Equal(t, api.NotifySubjectRelease, apiNL[0].Subject.Type)
		assert.Equal(t, "v0.0.2 is released", apiNL[0].Subject.Title)
		assert.True(t, apiNL[0].Unread)
		assert.False(t, apiNL[0].Pinned)
	})
}

func TestAPIRepoTransferNotification(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		session1 := loginUser(t, user2.Name)
		token1 := getTokenForLoggedInUser(t, session1, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		// create repo to move
		repoName := "moveME"
		apiRepo := new(api.Repository)
		req := NewRequestWithJSON(t, "POST", "/api/v1/user/repos", &api.CreateRepoOption{
			Name:        repoName,
			Description: "repo move around",
			Private:     false,
			Readme:      "Default",
			AutoInit:    true,
		}).AddTokenAuth(token1)
		resp := MakeRequest(t, req, http.StatusCreated)
		DecodeJSON(t, resp, apiRepo)

		defer func() {
			_ = repo_service.DeleteRepositoryDirectly(t.Context(), apiRepo.ID)
		}()

		// repo user1/moveME created, now transfer it to org6
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		session2 := loginUser(t, user2.Name)
		token2 := getTokenForLoggedInUser(t, session2, auth_model.AccessTokenScopeWriteRepository)
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/transfer", repo.OwnerName, repo.Name), &api.TransferRepoOption{
			NewOwner: "org6",
			TeamIDs:  nil,
		}).AddTokenAuth(token2)
		MakeRequest(t, req, http.StatusCreated)

		// user5 login to check notifications, because user5 is a member of org6's owners team
		user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
		session5 := loginUser(t, user5.Name)

		// Check notifications are as expected
		token5 := getTokenForLoggedInUser(t, session5, auth_model.AccessTokenScopeWriteNotification)
		req = NewRequest(t, "GET", "/api/v1/notifications?all=true").
			AddTokenAuth(token5)
		resp = MakeRequest(t, req, http.StatusOK)
		var apiNL []api.NotificationThread
		DecodeJSON(t, resp, &apiNL)

		assert.Equal(t, api.NotifySubjectRepository, apiNL[0].Subject.Type)
		assert.Equal(t, "user2/moveME", apiNL[0].Subject.Title)
		assert.True(t, apiNL[0].Unread)
		assert.False(t, apiNL[0].Pinned)
	})
}
