// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"
	"github.com/stretchr/testify/assert"
)

func TestAPICreateUserProject(t *testing.T) {
	createUserProjectSuccessTestCases := []struct {
		testName     string
		ctxUserID    int64
		doerID       int64
		title        string
		content      string
		templateType uint8
		cardType     uint8
	}{
		{
			testName:     "admin create project successfully",
			ctxUserID:    1,
			doerID:       1,
			title:        "site-admin",
			content:      "project_description",
			templateType: 1,
			cardType:     2,
		},
		{
			testName:     "user create project successfully",
			ctxUserID:    2,
			doerID:       2,
			title:        "user",
			content:      "project_description",
			templateType: 1,
			cardType:     2,
		},
	}

	createUserProjectFailTestCases := []struct {
		testName       string
		ctxUserID      int64
		doerID         int64
		title          string
		expectedStatus int
	}{
		{
			testName:       "failed to create project user is not admin and not owner",
			ctxUserID:      1,
			doerID:         2,
			title:          "user-not-admin-or-owner",
			expectedStatus: http.StatusForbidden,
		},
		{
			testName:       "project not created as title is empty",
			ctxUserID:      2,
			doerID:         2,
			title:          "",
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			testName:       "project not created as title is too long",
			ctxUserID:      2,
			doerID:         2,
			title:          "This is a very long title that will exceed the maximum allowed size of 100 characters. It keeps going beyond the limit.",
			expectedStatus: http.StatusUnprocessableEntity,
		},
	}

	defer tests.PrepareTestEnv(t)()

	for _, tt := range createUserProjectFailTestCases {
		t.Run(tt.testName, func(t *testing.T) {
			user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: tt.doerID})
			session := loginUser(t, user.Name)
			token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteAdmin, auth_model.AccessTokenScopeWriteUser)
			req := NewRequestWithJSON(t, "POST", "/api/v1/user/projects", &api.CreateProjectOption{
				Title: tt.title,
			}).AddTokenAuth(token)
			MakeRequest(t, req, tt.expectedStatus)
		})
	}

	for _, tt := range createUserProjectSuccessTestCases {
		t.Run(tt.testName, func(t *testing.T) {
			user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: tt.doerID})
			session := loginUser(t, user.Name)
			token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteAdmin, auth_model.AccessTokenScopeWriteUser)
			req := NewRequestWithJSON(t, "POST", "/api/v1/user/projects", &api.CreateProjectOption{
				Title:        tt.title,
				Content:      tt.content,
				TemplateType: tt.templateType,
				CardType:     tt.cardType,
			}).AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusCreated)
			var apiProject api.Project
			DecodeJSON(t, resp, &apiProject)
			assert.Equal(t, tt.title, apiProject.Title)
			assert.Equal(t, tt.content, apiProject.Description)
			assert.Equal(t, tt.templateType, apiProject.TemplateType)
			assert.Equal(t, tt.cardType, apiProject.CardType)
			assert.Equal(t, tt.ctxUserID, apiProject.OwnerID)
			assert.Equal(t, tt.doerID, apiProject.CreatorID)

			if tt.doerID != 1 {
				assert.Equal(t, apiProject.CreatorID, apiProject.OwnerID)
			}
		})
	}
}

func TestAPIGetUserProjects(t *testing.T) {

	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadAdmin, auth_model.AccessTokenScopeReadUser)

	expectedProjects := []*api.Project{
		{
			Title:        "project on user2",
			OwnerID:      2,
			IsClosed:     false,
			CreatorID:    2,
			TemplateType: 1,
			CardType:     2,
		},
		{
			Title:        "project without default column",
			OwnerID:      2,
			IsClosed:     false,
			CreatorID:    2,
			TemplateType: 1,
			CardType:     2,
		},
		{
			Title:        "project with multiple default columns",
			OwnerID:      2,
			IsClosed:     false,
			CreatorID:    2,
			TemplateType: 1,
			CardType:     2,
		},
	}

	t.Run("failed to get projects user not found", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/users/user-not-found/projects").AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})
	t.Run("get projects successfully", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/users/user2/projects").AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var apiProjects []*api.Project
		DecodeJSON(t, resp, &apiProjects)
		assert.Equal(t, len(expectedProjects), len(apiProjects))
		for i, expectedProject := range expectedProjects {
			assert.Equal(t, expectedProject.Title, apiProjects[i].Title)
			assert.Equal(t, expectedProject.OwnerID, apiProjects[i].OwnerID)
			assert.Equal(t, expectedProject.IsClosed, apiProjects[i].IsClosed)
			assert.Equal(t, expectedProject.CreatorID, apiProjects[i].CreatorID)
			assert.Equal(t, expectedProject.TemplateType, apiProjects[i].TemplateType)
			assert.Equal(t, expectedProject.CardType, apiProjects[i].CardType)
		}
	})
}
