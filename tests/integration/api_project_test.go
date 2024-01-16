// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/models/unittest"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"
	"github.com/stretchr/testify/assert"
)

func TestAPICreateUserProject(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	const title, description, board_type = "project_name", "project_description", uint8(project_model.BoardTypeBasicKanban)

	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteIssue, auth_model.AccessTokenScopeWriteUser)
	urlStr := fmt.Sprintf("/api/v1/user/projects?token=%s", token)

	req := NewRequestWithJSON(t, "POST", urlStr, &api.NewProjectPayload{
		Title:       title,
		Description: description,
		BoardType:   board_type,
	})
	resp := MakeRequest(t, req, http.StatusCreated)
	var apiProject api.Project
	DecodeJSON(t, resp, &apiProject)
	assert.Equal(t, title, apiProject.Title)
	assert.Equal(t, description, apiProject.Description)
	assert.Equal(t, board_type, apiProject.BoardType)
	assert.Equal(t, "user2", apiProject.Creator.UserName)
}

func TestAPICreateOrgProject(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	const title, description, board_type = "project_name", "project_description", uint8(project_model.BoardTypeBasicKanban)

	orgName := "org17"
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteIssue, auth_model.AccessTokenScopeWriteOrganization)
	urlStr := fmt.Sprintf("/api/v1/orgs/%s/projects?token=%s", orgName, token)

	req := NewRequestWithJSON(t, "POST", urlStr, &api.NewProjectPayload{
		Title:       title,
		Description: description,
		BoardType:   board_type,
	})
	resp := MakeRequest(t, req, http.StatusCreated)
	var apiProject api.Project
	DecodeJSON(t, resp, &apiProject)
	assert.Equal(t, title, apiProject.Title)
	assert.Equal(t, description, apiProject.Description)
	assert.Equal(t, board_type, apiProject.BoardType)
	assert.Equal(t, "org17", apiProject.Creator.UserName)
}

func TestAPICreateRepoProject(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	const title, description, board_type = "project_name", "project_description", uint8(project_model.BoardTypeBasicKanban)

	ownerName := "user2"
	repoName := "repo1"
	token := getUserToken(t, ownerName, auth_model.AccessTokenScopeWriteIssue, auth_model.AccessTokenScopeWriteOrganization)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/projects?token=%s", ownerName, repoName, token)

	req := NewRequestWithJSON(t, "POST", urlStr, &api.NewProjectPayload{
		Title:       title,
		Description: description,
		BoardType:   board_type,
	})
	resp := MakeRequest(t, req, http.StatusCreated)
	var apiProject api.Project
	DecodeJSON(t, resp, &apiProject)
	assert.Equal(t, title, apiProject.Title)
	assert.Equal(t, description, apiProject.Description)
	assert.Equal(t, board_type, apiProject.BoardType)
	assert.Equal(t, "repo1", apiProject.Repo.Name)
}

func TestAPIListUserProjects(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	token := getUserToken(t, "user2", auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopeReadIssue)
	link, _ := url.Parse(fmt.Sprintf("/api/v1/user/projects"))

	link.RawQuery = url.Values{"token": {token}}.Encode()

	req := NewRequest(t, "GET", link.String())
	var apiProjects []*api.Project

	resp := MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiProjects)
	assert.Len(t, apiProjects, 1)
}

func TestAPIListOrgProjects(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	orgName := "org17"
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeReadOrganization, auth_model.AccessTokenScopeReadIssue)
	link, _ := url.Parse(fmt.Sprintf("/api/v1/orgs/%s/projects", orgName))

	link.RawQuery = url.Values{"token": {token}}.Encode()

	req := NewRequest(t, "GET", link.String())
	var apiProjects []*api.Project

	resp := MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiProjects)
	assert.Len(t, apiProjects, 1)
}

func TestAPIListRepoProjects(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	ownerName := "user2"
	repoName := "repo1"
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopeReadIssue)
	link, _ := url.Parse(fmt.Sprintf("/api/v1/repos/%s/%s/projects", ownerName, repoName))

	link.RawQuery = url.Values{"token": {token}}.Encode()

	req := NewRequest(t, "GET", link.String())
	var apiProjects []*api.Project

	resp := MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiProjects)
	assert.Len(t, apiProjects, 1)
}

func TestAPIGetProject(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopeReadIssue)
	link, _ := url.Parse(fmt.Sprintf("/api/v1/projects/%d", 1))

	link.RawQuery = url.Values{"token": {token}}.Encode()

	req := NewRequest(t, "GET", link.String())
	var apiProject *api.Project

	resp := MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiProject)
	assert.Equal(t, "First project", apiProject.Title)
	assert.Equal(t, "repo1", apiProject.Repo.Name)
	assert.Equal(t, "user2", apiProject.Creator.UserName)
}

func TestAPIUpdateProject(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteIssue)
	link, _ := url.Parse(fmt.Sprintf("/api/v1/projects/%d", 1))

	link.RawQuery = url.Values{"token": {token}}.Encode()

	req := NewRequestWithJSON(t, "PATCH", link.String(), &api.UpdateProjectPayload{
		Title: "First project updated",
	})

	var apiProject *api.Project

	resp := MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiProject)
	assert.Equal(t, "First project updated", apiProject.Title)
}

func TestAPIDeleteProject(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteIssue)
	link, _ := url.Parse(fmt.Sprintf("/api/v1/projects/%d", 1))

	link.RawQuery = url.Values{"token": {token}}.Encode()

	req := NewRequest(t, "DELETE", link.String())

	MakeRequest(t, req, http.StatusNoContent)
	unittest.AssertNotExistsBean(t, &project_model.Project{ID: 1})
}
