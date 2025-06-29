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
	const title, description, boardType = "project_name", "project_description", uint8(project_model.TemplateTypeBasicKanban)

	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteIssue, auth_model.AccessTokenScopeWriteUser)

	req := NewRequestWithJSON(t, "POST", "/api/v1/user/projects", &api.NewProjectPayload{
		Title:       title,
		Description: description,
		BoardType:   boardType,
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)
	var apiProject api.Project
	DecodeJSON(t, resp, &apiProject)
	assert.Equal(t, title, apiProject.Title)
	assert.Equal(t, description, apiProject.Description)
	assert.Equal(t, boardType, apiProject.TemplateType)
	assert.Equal(t, "user2", apiProject.Creator.UserName)
}

func TestAPICreateOrgProject(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	const title, description, boardType = "project_name", "project_description", uint8(project_model.TemplateTypeBasicKanban)

	orgName := "org17"
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteIssue, auth_model.AccessTokenScopeWriteOrganization)
	urlStr := fmt.Sprintf("/api/v1/orgs/%s/projects", orgName)

	req := NewRequestWithJSON(t, "POST", urlStr, &api.NewProjectPayload{
		Title:       title,
		Description: description,
		BoardType:   boardType,
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)
	var apiProject api.Project
	DecodeJSON(t, resp, &apiProject)
	assert.Equal(t, title, apiProject.Title)
	assert.Equal(t, description, apiProject.Description)
	assert.Equal(t, boardType, apiProject.TemplateType)
	assert.Equal(t, "user2", apiProject.Creator.UserName)
	assert.Equal(t, "org17", apiProject.Owner.UserName)
}

func TestAPICreateRepoProject(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	const title, description, boardType = "project_name", "project_description", uint8(project_model.TemplateTypeBasicKanban)

	ownerName := "user2"
	repoName := "repo1"
	token := getUserToken(t, ownerName, auth_model.AccessTokenScopeWriteIssue, auth_model.AccessTokenScopeWriteOrganization)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/projects", ownerName, repoName)

	req := NewRequestWithJSON(t, "POST", urlStr, &api.NewProjectPayload{
		Title:       title,
		Description: description,
		BoardType:   boardType,
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)
	var apiProject api.Project
	DecodeJSON(t, resp, &apiProject)
	assert.Equal(t, title, apiProject.Title)
	assert.Equal(t, description, apiProject.Description)
	assert.Equal(t, boardType, apiProject.TemplateType)
	assert.Equal(t, "repo1", apiProject.Repo.Name)
}

func TestAPIListUserProjects(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	token := getUserToken(t, "user2", auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopeReadIssue)
	link, _ := url.Parse("/api/v1/user/projects")

	req := NewRequest(t, "GET", link.String()).AddTokenAuth(token)
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

	req := NewRequest(t, "GET", link.String()).AddTokenAuth(token)
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

	req := NewRequest(t, "GET", link.String()).AddTokenAuth(token)
	var apiProjects []*api.Project

	resp := MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiProjects)
	assert.Len(t, apiProjects, 1)
}

func TestAPIGetProject(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopeReadIssue)
	link, _ := url.Parse(fmt.Sprintf("/api/v1/projects/%d", 1))

	req := NewRequest(t, "GET", link.String()).AddTokenAuth(token)
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

	req := NewRequestWithJSON(t, "PATCH", link.String(), &api.UpdateProjectPayload{Title: "First project updated"}).AddTokenAuth(token)

	var apiProject *api.Project

	resp := MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiProject)
	assert.Equal(t, "First project updated", apiProject.Title)
}

func TestAPIDeleteProject(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteIssue)
	link, _ := url.Parse(fmt.Sprintf("/api/v1/projects/%d", 1))

	req := NewRequest(t, "DELETE", link.String()).AddTokenAuth(token)

	MakeRequest(t, req, http.StatusNoContent)
	unittest.AssertNotExistsBean(t, &project_model.Project{ID: 1})
}
