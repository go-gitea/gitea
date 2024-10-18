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

func TestAPICreateRepoProject(t *testing.T) {
	createRepoProjectSuccessTestCases := []struct {
		testName     string
		ownerName    string
		repoName     string
		repoID       int64
		doerID       int64
		title        string
		content      string
		templateType uint8
		cardType     uint8
	}{
		{
			testName:     "member create project successfully with write access",
			ownerName:    "org3",
			repoName:     "repo3",
			repoID:       3,
			doerID:       4,
			title:        "member-with-write-access",
			content:      "project_description",
			templateType: 1,
			cardType:     2,
		},
		{
			testName:     "collaborator create project successfully with write access",
			ownerName:    "privated_org",
			repoName:     "public_repo_on_private_org",
			repoID:       40,
			doerID:       4,
			title:        "collaborator-with-write-access",
			content:      "project_description",
			templateType: 1,
			cardType:     2,
		},
	}

	createRepoProjectFailTestCases := []struct {
		testName       string
		ownerName      string
		repoName       string
		repoID         int64
		doerID         int64
		title          string
		expectedStatus int
	}{
		{
			testName:       "user is not in organization",
			ownerName:      "org3",
			repoName:       "repo3",
			repoID:         3,
			doerID:         5,
			title:          "user-not-in-org",
			expectedStatus: http.StatusForbidden,
		},
		{
			testName:       "user is not collaborator",
			ownerName:      "org3",
			repoName:       "repo3",
			repoID:         3,
			doerID:         4,
			title:          "user-not-collaborator",
			expectedStatus: http.StatusForbidden,
		},
		{
			testName:       "user is member but not sufficient access",
			ownerName:      "org17",
			repoName:       "big_test_private_4",
			repoID:         24,
			doerID:         20,
			title:          "member-not-sufficient-access",
			expectedStatus: http.StatusForbidden,
		},
		{
			testName:       "project not created as title is empty",
			ownerName:      "org3",
			repoName:       "repo3",
			repoID:         3,
			doerID:         2,
			title:          "",
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			testName:       "project not created as title is too long",
			ownerName:      "org3",
			repoName:       "repo3",
			repoID:         3,
			doerID:         2,
			title:          "This is a very long title that will exceed the maximum allowed size of 100 characters. It keeps going beyond the limit.",
			expectedStatus: http.StatusUnprocessableEntity,
		},
	}

	defer tests.PrepareTestEnv(t)()

	for _, tt := range createRepoProjectFailTestCases {
		t.Run(tt.testName, func(t *testing.T) {
			user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: tt.doerID})
			session := loginUser(t, user.Name)
			token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteAdmin, auth_model.AccessTokenScopeWriteRepository)
			req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/projects", tt.ownerName, tt.repoName), &api.CreateProjectOption{
				Title: tt.title,
			}).AddTokenAuth(token)
			MakeRequest(t, req, tt.expectedStatus)
		})
	}

	for _, tt := range createRepoProjectSuccessTestCases {
		t.Run(tt.testName, func(t *testing.T) {
			user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: tt.doerID})
			session := loginUser(t, user.Name)
			token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteAdmin, auth_model.AccessTokenScopeWriteRepository)
			req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/projects", tt.ownerName, tt.repoName), &api.CreateProjectOption{
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
			assert.Equal(t, tt.repoID, apiProject.RepoID)
			assert.Equal(t, tt.doerID, apiProject.CreatorID)
		})
	}
}

func TestAPIGetRepoProjects(t *testing.T) {

	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadAdmin, auth_model.AccessTokenScopeReadRepository)

	expectedProjects := []*api.Project{
		{
			Title:        "First Project",
			RepoID:       1,
			IsClosed:     false,
			CreatorID:    2,
			TemplateType: 1,
			CardType:     2,
		},
		{
			Title:        "project2 on repo1",
			RepoID:       1,
			IsClosed:     false,
			CreatorID:    2,
			TemplateType: 1,
			CardType:     2,
		},
	}

	t.Run("failed to get projects repo not found", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/repos/user2/repo-not-found/projects").AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})
	t.Run("get projects successfully", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/repos/user2/repo1/projects").AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var apiProjects []*api.Project
		DecodeJSON(t, resp, &apiProjects)
		assert.Equal(t, len(expectedProjects), len(apiProjects))
		for i, expectedProject := range expectedProjects {
			assert.Equal(t, expectedProject.Title, apiProjects[i].Title)
			assert.Equal(t, expectedProject.RepoID, apiProjects[i].RepoID)
			assert.Equal(t, expectedProject.IsClosed, apiProjects[i].IsClosed)
			assert.Equal(t, expectedProject.CreatorID, apiProjects[i].CreatorID)
			assert.Equal(t, expectedProject.TemplateType, apiProjects[i].TemplateType)
			assert.Equal(t, expectedProject.CardType, apiProjects[i].CardType)
		}
	})
}

func TestAPIUpdateIssueProject(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteAdmin, auth_model.AccessTokenScopeWriteRepository)

	req := NewRequestWithJSON(t, "PUT", "/api/v1/repos/user2/repo1/projects/issues", &api.UpdateIssuesOption{
		ProjectID: 10,
		Issues:    []int64{1, 2},
	}).AddTokenAuth(token)

	MakeRequest(t, req, http.StatusNoContent)
}
