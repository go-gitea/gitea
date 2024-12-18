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

func TestAPIGetProject(t *testing.T) {
	getProjectTestCases := []struct {
		testName       string
		projectID      int64
		expectedStatus int
	}{
		{
			testName:       "get project successfully",
			projectID:      1,
			expectedStatus: http.StatusOK,
		},
		{
			testName:       "project not found",
			projectID:      20,
			expectedStatus: http.StatusNotFound,
		},
	}

	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadAdmin, auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopeReadOrganization)

	for _, tt := range getProjectTestCases {
		t.Run(tt.testName, func(t *testing.T) {
			req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/projects/%d", tt.projectID)).AddTokenAuth(token)
			MakeRequest(t, req, tt.expectedStatus)
		})
	}
}

func TestAPIEditProject(t *testing.T) {
	editProjectFailTestCases := []struct {
		testName       string
		projectID      int64
		expectedStatus int
	}{
		{
			testName:       "repo is archived",
			projectID:      7,
			expectedStatus: http.StatusLocked,
		},
		{
			testName:       "insufficient access",
			projectID:      2,
			expectedStatus: http.StatusForbidden,
		},
	}

	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteAdmin, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteOrganization)

	for _, tt := range editProjectFailTestCases {
		t.Run(tt.testName, func(t *testing.T) {
			req := NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/projects/%d", tt.projectID),
				&api.EditProjectOption{
					Title: "title",
				}).AddTokenAuth(token)
			MakeRequest(t, req, tt.expectedStatus)
		})
	}

	t.Run("edit project successfully", func(t *testing.T) {
		expectedProject := api.Project{
			Title:       "new title",
			Description: "new content",
		}
		req := NewRequestWithJSON(t, "PATCH", "/api/v1/projects/1", &api.EditProjectOption{
			Title:   expectedProject.Title,
			Content: expectedProject.Description,
		}).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var apiProject api.Project
		DecodeJSON(t, resp, &apiProject)
		assert.Equal(t, expectedProject.Title, apiProject.Title)
		assert.Equal(t, expectedProject.Description, apiProject.Description)
	})
}

func TestAPIDeleteProject(t *testing.T) {

	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteAdmin, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteOrganization)

	t.Run("delete project successfully", func(t *testing.T) {
		req := NewRequest(t, "DELETE", "/api/v1/projects/1").AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)
	})
}

func TestAPIChangeProjectStatus(t *testing.T) {

	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteAdmin, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteOrganization)

	t.Run("change project status successfully", func(t *testing.T) {
		req := NewRequest(t, "PATCH", "/api/v1/projects/1/close").AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var apiProject api.Project
		DecodeJSON(t, resp, &apiProject)
		assert.Equal(t, true, apiProject.IsClosed)
	})
}
