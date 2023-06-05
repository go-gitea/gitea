// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestOAuth2Application(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	testAPICreateOAuth2Application(t)
	testAPIListOAuth2Applications(t)
	testAPIGetOAuth2Application(t)
	testAPIUpdateOAuth2Application(t)
	testAPIDeleteOAuth2Application(t)
}

func testAPICreateOAuth2Application(t *testing.T) {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	appBody := api.CreateOAuth2ApplicationOptions{
		Name: "test-app-1",
		RedirectURIs: []string{
			"http://www.google.com",
		},
		ConfidentialClient: true,
	}

	req := NewRequestWithJSON(t, "POST", "/api/v1/user/applications/oauth2", &appBody)
	req = AddBasicAuthHeader(req, user.Name)
	resp := MakeRequest(t, req, http.StatusCreated)

	var createdApp *api.OAuth2Application
	DecodeJSON(t, resp, &createdApp)

	assert.EqualValues(t, appBody.Name, createdApp.Name)
	assert.Len(t, createdApp.ClientSecret, 56)
	assert.Len(t, createdApp.ClientID, 36)
	assert.True(t, createdApp.ConfidentialClient)
	assert.NotEmpty(t, createdApp.Created)
	assert.EqualValues(t, appBody.RedirectURIs[0], createdApp.RedirectURIs[0])
	unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Application{UID: user.ID, Name: createdApp.Name})
}

func testAPIListOAuth2Applications(t *testing.T) {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser)

	existApp := unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Application{
		UID:  user.ID,
		Name: "test-app-1",
		RedirectURIs: []string{
			"http://www.google.com",
		},
		ConfidentialClient: true,
	})

	urlStr := fmt.Sprintf("/api/v1/user/applications/oauth2?token=%s", token)
	req := NewRequest(t, "GET", urlStr)
	resp := MakeRequest(t, req, http.StatusOK)

	var appList api.OAuth2ApplicationList
	DecodeJSON(t, resp, &appList)
	expectedApp := appList[0]

	assert.EqualValues(t, existApp.Name, expectedApp.Name)
	assert.EqualValues(t, existApp.ClientID, expectedApp.ClientID)
	assert.Equal(t, existApp.ConfidentialClient, expectedApp.ConfidentialClient)
	assert.Len(t, expectedApp.ClientID, 36)
	assert.Empty(t, expectedApp.ClientSecret)
	assert.EqualValues(t, existApp.RedirectURIs[0], expectedApp.RedirectURIs[0])
	unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Application{ID: expectedApp.ID, Name: expectedApp.Name})
}

func testAPIDeleteOAuth2Application(t *testing.T) {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteUser)

	oldApp := unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Application{
		UID:  user.ID,
		Name: "test-app-1",
	})

	urlStr := fmt.Sprintf("/api/v1/user/applications/oauth2/%d?token=%s", oldApp.ID, token)
	req := NewRequest(t, "DELETE", urlStr)
	MakeRequest(t, req, http.StatusNoContent)

	unittest.AssertNotExistsBean(t, &auth_model.OAuth2Application{UID: oldApp.UID, Name: oldApp.Name})

	// Delete again will return not found
	req = NewRequest(t, "DELETE", urlStr)
	MakeRequest(t, req, http.StatusNotFound)
}

func testAPIGetOAuth2Application(t *testing.T) {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser)

	existApp := unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Application{
		UID:  user.ID,
		Name: "test-app-1",
		RedirectURIs: []string{
			"http://www.google.com",
		},
		ConfidentialClient: true,
	})

	urlStr := fmt.Sprintf("/api/v1/user/applications/oauth2/%d?token=%s", existApp.ID, token)
	req := NewRequest(t, "GET", urlStr)
	resp := MakeRequest(t, req, http.StatusOK)

	var app api.OAuth2Application
	DecodeJSON(t, resp, &app)
	expectedApp := app

	assert.EqualValues(t, existApp.Name, expectedApp.Name)
	assert.EqualValues(t, existApp.ClientID, expectedApp.ClientID)
	assert.Equal(t, existApp.ConfidentialClient, expectedApp.ConfidentialClient)
	assert.Len(t, expectedApp.ClientID, 36)
	assert.Empty(t, expectedApp.ClientSecret)
	assert.Len(t, expectedApp.RedirectURIs, 1)
	assert.EqualValues(t, existApp.RedirectURIs[0], expectedApp.RedirectURIs[0])
	unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Application{ID: expectedApp.ID, Name: expectedApp.Name})
}

func testAPIUpdateOAuth2Application(t *testing.T) {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	existApp := unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Application{
		UID:  user.ID,
		Name: "test-app-1",
		RedirectURIs: []string{
			"http://www.google.com",
		},
	})

	appBody := api.CreateOAuth2ApplicationOptions{
		Name: "test-app-1",
		RedirectURIs: []string{
			"http://www.google.com/",
			"http://www.github.com/",
		},
		ConfidentialClient: true,
	}

	urlStr := fmt.Sprintf("/api/v1/user/applications/oauth2/%d", existApp.ID)
	req := NewRequestWithJSON(t, "PATCH", urlStr, &appBody)
	req = AddBasicAuthHeader(req, user.Name)
	resp := MakeRequest(t, req, http.StatusOK)

	var app api.OAuth2Application
	DecodeJSON(t, resp, &app)
	expectedApp := app

	assert.Len(t, expectedApp.RedirectURIs, 2)
	assert.EqualValues(t, expectedApp.RedirectURIs[0], appBody.RedirectURIs[0])
	assert.EqualValues(t, expectedApp.RedirectURIs[1], appBody.RedirectURIs[1])
	assert.Equal(t, expectedApp.ConfidentialClient, appBody.ConfidentialClient)
	unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Application{ID: expectedApp.ID, Name: expectedApp.Name})
}
