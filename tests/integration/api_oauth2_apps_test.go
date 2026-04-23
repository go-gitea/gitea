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
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	redirectURIs := []string{"http://www.google.com", "my-app:foo"}
	appBody := api.CreateOAuth2ApplicationOptions{Name: "test-app-1", RedirectURIs: redirectURIs, ConfidentialClient: true}

	// no custom scheme
	req := NewRequestWithJSON(t, "POST", "/api/v1/user/applications/oauth2", &appBody).AddBasicAuth(user.Name)
	MakeRequest(t, req, http.StatusBadRequest)

	// with custom scheme
	defer test.MockVariableValue(&setting.OAuth2.CustomSchemes, []string{"my-app"})()
	req = NewRequestWithJSON(t, "POST", "/api/v1/user/applications/oauth2", &appBody).AddBasicAuth(user.Name)
	resp := MakeRequest(t, req, http.StatusCreated)
	createdApp := DecodeJSON(t, resp, &api.OAuth2Application{})

	assert.Equal(t, appBody.Name, createdApp.Name)
	assert.Len(t, createdApp.ClientSecret, 56)
	assert.Len(t, createdApp.ClientID, 36)
	assert.True(t, createdApp.ConfidentialClient)
	assert.NotEmpty(t, createdApp.Created)
	assert.Equal(t, redirectURIs, createdApp.RedirectURIs)
	unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Application{UID: user.ID, Name: createdApp.Name})
}

func testAPIListOAuth2Applications(t *testing.T) {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser)

	existApp := unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Application{UID: user.ID, Name: "test-app-1", ConfidentialClient: true})
	require.NotEmpty(t, existApp.RedirectURIs)

	req := NewRequest(t, "GET", "/api/v1/user/applications/oauth2").AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	appList := DecodeJSON(t, resp, api.OAuth2ApplicationList{})
	expectedApp := appList[0]

	assert.Equal(t, expectedApp.Name, existApp.Name)
	assert.Equal(t, expectedApp.ClientID, existApp.ClientID)
	assert.Equal(t, expectedApp.ConfidentialClient, existApp.ConfidentialClient)
	assert.Len(t, expectedApp.ClientID, 36)
	assert.Empty(t, expectedApp.ClientSecret)
	assert.Equal(t, expectedApp.RedirectURIs, existApp.RedirectURIs)
	unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Application{ID: expectedApp.ID, Name: expectedApp.Name})
}

func testAPIDeleteOAuth2Application(t *testing.T) {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteUser)

	oldApp := unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Application{UID: user.ID, Name: "test-app-1"})

	urlStr := fmt.Sprintf("/api/v1/user/applications/oauth2/%d", oldApp.ID)
	req := NewRequest(t, "DELETE", urlStr).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	unittest.AssertNotExistsBean(t, &auth_model.OAuth2Application{UID: oldApp.UID, Name: oldApp.Name})

	// Delete again will return not found
	req = NewRequest(t, "DELETE", urlStr).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func testAPIGetOAuth2Application(t *testing.T) {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser)

	existApp := unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Application{UID: user.ID, Name: "test-app-1", ConfidentialClient: true})
	require.NotEmpty(t, existApp.RedirectURIs)

	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/user/applications/oauth2/%d", existApp.ID)).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	expectedApp := DecodeJSON(t, resp, &api.OAuth2Application{})

	assert.Equal(t, expectedApp.Name, existApp.Name)
	assert.Equal(t, expectedApp.ClientID, existApp.ClientID)
	assert.Equal(t, expectedApp.ConfidentialClient, existApp.ConfidentialClient)
	assert.Len(t, expectedApp.ClientID, 36)
	assert.Empty(t, expectedApp.ClientSecret)
	assert.Equal(t, expectedApp.RedirectURIs, existApp.RedirectURIs)
	unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Application{ID: expectedApp.ID, Name: expectedApp.Name})
}

func testAPIUpdateOAuth2Application(t *testing.T) {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	existApp := unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Application{UID: user.ID, Name: "test-app-1"})
	redirectURIs := []string{"https://www.google.com", "my-app:foo"}
	appBody := api.CreateOAuth2ApplicationOptions{Name: "test-app-1", RedirectURIs: redirectURIs, ConfidentialClient: true}
	urlStr := fmt.Sprintf("/api/v1/user/applications/oauth2/%d", existApp.ID)

	// no custom scheme
	req := NewRequestWithJSON(t, "PATCH", urlStr, &appBody).AddBasicAuth(user.Name)
	MakeRequest(t, req, http.StatusBadRequest)

	// with custom scheme
	defer test.MockVariableValue(&setting.OAuth2.CustomSchemes, []string{"my-app"})()
	req = NewRequestWithJSON(t, "PATCH", urlStr, &appBody).AddBasicAuth(user.Name)
	resp := MakeRequest(t, req, http.StatusOK)

	expectedApp := DecodeJSON(t, resp, &api.OAuth2Application{})
	assert.Equal(t, expectedApp.RedirectURIs, appBody.RedirectURIs)
	assert.Equal(t, expectedApp.ConfidentialClient, appBody.ConfidentialClient)
	unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Application{ID: expectedApp.ID, Name: expectedApp.Name})
}
