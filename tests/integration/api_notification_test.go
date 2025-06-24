// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	activities_model "code.gitea.io/gitea/models/activities"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPINotification(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	thread5 := unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{ID: 5})
	assert.NoError(t, thread5.LoadAttributes(db.DefaultContext))
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
	assert.Equal(t, thread5.Issue.APIURL(db.DefaultContext), apiN.Subject.URL)
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
	assert.NoError(t, thread5.LoadAttributes(db.DefaultContext))
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
