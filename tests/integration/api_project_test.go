// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIListRepositoryProjects(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)
	link, _ := url.Parse(fmt.Sprintf("/api/v1/repos/%s/%s/projects", owner.Name, repo.Name))

	link.RawQuery = url.Values{"token": {token}}.Encode()

	req := NewRequest(t, "GET", link.String())
	var apiProjects []*api.Project

	resp := session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiProjects)

	assert.Len(t, apiProjects, 2)
	for _, apiProject := range apiProjects {
		unittest.AssertExistsAndLoadBean(t, &project_model.Project{ID: apiProject.ID, RepoID: repo.ID, IsClosed: apiProject.IsClosed})
	}
}

func TestAPICreateRepositoryProject(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	const title, description, boardType = "project_name", "project_description", uint8(project_model.BoardTypeBasicKanban)

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/projects?token=%s", owner.Name, repo.Name, token)

	req := NewRequestWithJSON(t, "POST", urlStr, &api.NewProjectPayload{
		Title:       title,
		Description: description,
		BoardType:   boardType,
	})
	resp := session.MakeRequest(t, req, http.StatusCreated)
	var apiProject api.Project
	DecodeJSON(t, resp, &apiProject)
	assert.Equal(t, title, apiProject.Title)
	assert.Equal(t, description, apiProject.Description)
	assert.Equal(t, boardType, apiProject.BoardType)
	assert.Equal(t, owner.Name, apiProject.Creator.UserName)
	assert.Equal(t, owner.FullName, apiProject.Creator.FullName)
	assert.Equal(t, repo.ID, apiProject.Repo.ID)
	assert.Equal(t, repo.Name, apiProject.Repo.Name)
	assert.Equal(t, repo.FullName(), apiProject.Repo.FullName)
	assert.Equal(t, owner.Name, apiProject.Repo.Owner)
}

func TestAPIGetProject(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	project := unittest.AssertExistsAndLoadBean(t, &project_model.Project{ID: 1})
	projectRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: project.RepoID})
	projectCreator := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: project.CreatorID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)

	urlStr := fmt.Sprintf("/api/v1/projects/%d?token=%s", project.ID, token)

	req := NewRequest(t, "GET", urlStr)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var apiProject api.Project
	DecodeJSON(t, resp, &apiProject)

	assert.Equal(t, project.Title, apiProject.Title)
	assert.Equal(t, project.Description, apiProject.Description)
	assert.Equal(t, uint8(project.BoardType), apiProject.BoardType)
	assert.Equal(t, projectCreator.Name, apiProject.Creator.UserName)
	assert.Equal(t, projectCreator.FullName, apiProject.Creator.FullName)
	assert.Equal(t, projectRepo.ID, apiProject.Repo.ID)
	assert.Equal(t, projectRepo.Name, apiProject.Repo.Name)
	assert.Equal(t, projectRepo.FullName(), apiProject.Repo.FullName)
	assert.Equal(t, projectRepo.OwnerName, apiProject.Repo.Owner)
}

func TestAPIUpdateProject(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	projectBefore := unittest.AssertExistsAndLoadBean(t, &project_model.Project{ID: 1})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)

	urlStr := fmt.Sprintf("/api/v1/projects/%d?token=%s", projectBefore.ID, token)

	req := NewRequestWithJSON(t, "PATCH", urlStr, &api.UpdateProjectPayload{
		Title:       "This is new title",
		Description: "This is new description",
	})
	resp := session.MakeRequest(t, req, http.StatusOK)

	var apiProject api.Project
	DecodeJSON(t, resp, &apiProject)
	projectAfter := unittest.AssertExistsAndLoadBean(t, &project_model.Project{ID: 1})

	assert.Equal(t, "This is new title", apiProject.Title)
	assert.Equal(t, "This is new description", apiProject.Description)
	assert.Equal(t, "This is new title", projectAfter.Title)
	assert.Equal(t, "This is new description", projectAfter.Description)
}

func TestAPIDeleteProject(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	projectBefore := unittest.AssertExistsAndLoadBean(t, &project_model.Project{ID: 1})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)

	urlStr := fmt.Sprintf("/api/v1/projects/%d?token=%s", projectBefore.ID, token)

	req := NewRequest(t, "DELETE", urlStr)
	_ = session.MakeRequest(t, req, http.StatusNoContent)

	unittest.AssertNotExistsBean(t, &project_model.Project{ID: 1})
}
