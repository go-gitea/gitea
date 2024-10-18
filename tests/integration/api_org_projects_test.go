// Copyright 2017 The Gitea Authors. All rights reserved.
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

func TestAPICreateOrgProject(t *testing.T) {
	createOrgProjectSuccessTestCases := []struct {
		testName     string
		orgName      string
		ctxUserID    int64
		doerID       int64
		title        string
		content      string
		templateType uint8
		cardType     uint8
	}{
		{
			testName:     "site admin create project successfully",
			ctxUserID:    3,
			doerID:       1,
			title:        "site-admin",
			content:      "project_description",
			templateType: 1,
			cardType:     2,
		},
		{
			testName:     "org owner create project successfully",
			ctxUserID:    3,
			doerID:       2,
			title:        "org-owner",
			content:      "project_description",
			templateType: 1,
			cardType:     2,
		},
		{
			testName:     "member create project successfully with write access",
			ctxUserID:    3,
			doerID:       4,
			title:        "member-with-write-access",
			content:      "project_description",
			templateType: 1,
			cardType:     2,
		},
	}

	createOrgProjectFailTestCases := []struct {
		testName       string
		orgName        string
		ctxUserID      int64
		doerID         int64
		title          string
		expectedStatus int
	}{
		{
			testName:       "user is not in organization",
			orgName:        "org3",
			ctxUserID:      3,
			doerID:         5,
			title:          "user-not-in-org",
			expectedStatus: http.StatusForbidden,
		},
		{
			testName:       "user is member but not sufficient access",
			orgName:        "org17",
			ctxUserID:      17,
			doerID:         20,
			title:          "member-not-sufficient-access",
			expectedStatus: http.StatusForbidden,
		},
		{
			testName:       "project not created as title is empty",
			orgName:        "org3",
			ctxUserID:      3,
			doerID:         2,
			title:          "",
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			testName:       "project not created as title is too long",
			orgName:        "org3",
			ctxUserID:      3,
			doerID:         2,
			title:          "This is a very long title that will exceed the maximum allowed size of 100 characters. It keeps going beyond the limit.",
			expectedStatus: http.StatusUnprocessableEntity,
		},
	}

	defer tests.PrepareTestEnv(t)()

	for _, tt := range createOrgProjectFailTestCases {
		t.Run(tt.testName, func(t *testing.T) {
			user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: tt.doerID})
			session := loginUser(t, user.Name)
			token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteAdmin, auth_model.AccessTokenScopeWriteOrganization)
			req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/projects", tt.orgName), &api.CreateProjectOption{
				Title: tt.title,
			}).AddTokenAuth(token)
			MakeRequest(t, req, tt.expectedStatus)
		})
	}

	for _, tt := range createOrgProjectSuccessTestCases {
		t.Run(tt.testName, func(t *testing.T) {
			user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: tt.doerID})
			session := loginUser(t, user.Name)
			token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteAdmin, auth_model.AccessTokenScopeWriteOrganization)
			req := NewRequestWithJSON(t, "POST", "/api/v1/orgs/org3/projects", &api.CreateProjectOption{
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
		})
	}
}

func TestAPIGetOrgProjects(t *testing.T) {

	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadAdmin, auth_model.AccessTokenScopeReadOrganization)

	expectedProjects := []*api.Project{
		{
			Title:        "project1 belongs to org3",
			OwnerID:      3,
			IsClosed:     true,
			CreatorID:    3,
			TemplateType: 1,
			CardType:     2,
		},
		{
			Title:        "project2 belongs to org3",
			OwnerID:      3,
			IsClosed:     false,
			CreatorID:    3,
			TemplateType: 1,
			CardType:     2,
		},
	}

	t.Run("failed to get projects org not found", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/orgs/org90/projects").AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})
	t.Run("get projects successfully", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/orgs/org3/projects").AddTokenAuth(token)
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
