// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.package models

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestOAuth2Application(t *testing.T) {
	defer prepareTestEnv(t)()
	testAPICreateOAuth2Application(t)
	testAPIListOAuth2Applications(t)
	testAPIDeleteOAuth2Application(t)
}

func testAPICreateOAuth2Application(t *testing.T) {
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	appBody := api.CreateOAuth2ApplicationOptions{
		Name: "test-app-1",
		RedirectURIs: []string{
			"http://www.google.com",
		},
	}

	req := NewRequestWithJSON(t, "POST", "/api/v1/user/applications/oauth2", &appBody)
	req = AddBasicAuthHeader(req, user.Name)
	resp := MakeRequest(t, req, http.StatusCreated)

	var createdApp *api.OAuth2Application
	DecodeJSON(t, resp, &createdApp)

	assert.EqualValues(t, appBody.Name, createdApp.Name)
	assert.Len(t, createdApp.ClientSecret, 44)
	assert.Len(t, createdApp.ClientID, 36)
	assert.NotEmpty(t, createdApp.Created)
	assert.EqualValues(t, appBody.RedirectURIs[0], createdApp.RedirectURIs[0])
	models.AssertExistsAndLoadBean(t, &models.OAuth2Application{UID: user.ID, Name: createdApp.Name})
}

func testAPIListOAuth2Applications(t *testing.T) {
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session)

	existApp := models.AssertExistsAndLoadBean(t, &models.OAuth2Application{
		UID:  user.ID,
		Name: "test-app-1",
		RedirectURIs: []string{
			"http://www.google.com",
		},
	}).(*models.OAuth2Application)

	urlStr := fmt.Sprintf("/api/v1/user/applications/oauth2?token=%s", token)
	req := NewRequest(t, "GET", urlStr)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var appList api.OAuth2ApplicationList
	DecodeJSON(t, resp, &appList)
	expectedApp := appList[0]

	assert.EqualValues(t, existApp.Name, expectedApp.Name)
	assert.EqualValues(t, existApp.ClientID, expectedApp.ClientID)
	assert.Len(t, expectedApp.ClientID, 36)
	assert.Empty(t, expectedApp.ClientSecret)
	assert.EqualValues(t, existApp.RedirectURIs[0], expectedApp.RedirectURIs[0])
	models.AssertExistsAndLoadBean(t, &models.OAuth2Application{ID: expectedApp.ID, Name: expectedApp.Name})
}

func testAPIDeleteOAuth2Application(t *testing.T) {
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session)

	oldApp := models.AssertExistsAndLoadBean(t, &models.OAuth2Application{
		UID:  user.ID,
		Name: "test-app-1",
		RedirectURIs: []string{
			"http://www.google.com",
		},
	}).(*models.OAuth2Application)

	urlStr := fmt.Sprintf("/api/v1/user/applications/oauth2/%d?token=%s", oldApp.ID, token)
	req := NewRequest(t, "DELETE", urlStr)
	session.MakeRequest(t, req, http.StatusNoContent)

	models.AssertNotExistsBean(t, &models.OAuth2Application{UID: oldApp.UID, Name: oldApp.Name})
}
