// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration

import (
	"fmt"
	"net/http"
	"testing"

	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIListProjectBoads(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)

	urlStr := fmt.Sprintf("/api/v1/projects/%d/boards?token=%s", 1, token)

	req := NewRequest(t, "GET", urlStr)
	resp := MakeRequest(t, req, http.StatusOK)

	var apiProjectBoards []*api.ProjectBoard
	DecodeJSON(t, resp, &apiProjectBoards)

	// We include the Uncategorized board by default, so check the len accordingly
	assert.Len(t, apiProjectBoards, unittest.GetCount(t, &project_model.Board{})+1)
}

func TestAPICreateProjectBoard(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	title, isDefault, color := "Board 10", false, "#ff0000"

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	project := unittest.AssertExistsAndLoadBean(t, &project_model.Project{ID: 1})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)

	urlStr := fmt.Sprintf("/api/v1/projects/%d/boards?token=%s", project.ID, token)

	req := NewRequestWithJSON(t, "POST", urlStr, &api.NewProjectBoardPayload{
		Title:   title,
		Default: isDefault,
		Color:   color,
		Sorting: 0,
	})
	resp := session.MakeRequest(t, req, http.StatusCreated)
	var apiProjectBoard *api.ProjectBoard
	DecodeJSON(t, resp, &apiProjectBoard)

	unittest.AssertExistsAndLoadBean(t, &project_model.Board{ID: apiProjectBoard.ID})
	assert.Equal(t, title, apiProjectBoard.Title)
	assert.Equal(t, isDefault, apiProjectBoard.Default)
	assert.Equal(t, color, apiProjectBoard.Color)
}

func TestAPIGetProjectBoard(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	board := unittest.AssertExistsAndLoadBean(t, &project_model.Board{ID: 1})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)

	urlStr := fmt.Sprintf("/api/v1/projects/boards/%d?token=%s", board.ID, token)
	req := NewRequest(t, "GET", urlStr)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var apiProjectBoard *api.ProjectBoard
	DecodeJSON(t, resp, &apiProjectBoard)

	assert.Equal(t, apiProjectBoard.Title, board.Title)
	assert.Equal(t, apiProjectBoard.Default, board.Default)
	assert.Equal(t, apiProjectBoard.Color, board.Color)
	assert.Equal(t, apiProjectBoard.Sorting, board.Sorting)
}

func TestAPIUpdateProjectBoard(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	board := unittest.AssertExistsAndLoadBean(t, &project_model.Board{ID: 1})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)

	urlStr := fmt.Sprintf("/api/v1/projects/boards/%d?token=%s", board.ID, token)
	payload := &api.UpdateProjectBoardPayload{
		Title: "Backlog",
		Color: "#ffaa00",
	}
	req := NewRequestWithJSON(t, "PATCH", urlStr, payload)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var apiProjectBoard *api.ProjectBoard
	DecodeJSON(t, resp, &apiProjectBoard)

	assert.Equal(t, payload.Title, apiProjectBoard.Title)
	assert.Equal(t, payload.Color, apiProjectBoard.Color)

	boardAfter := unittest.AssertExistsAndLoadBean(t, &project_model.Board{ID: 1})
	assert.Equal(t, payload.Title, boardAfter.Title)
	assert.Equal(t, payload.Color, boardAfter.Color)
}

func TestAPIDeleteProjectBoard(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)

	urlStr := fmt.Sprintf("/api/v1/projects/boards/%d?token=%s", 1, token)
	req := NewRequest(t, "DELETE", urlStr)
	_ = session.MakeRequest(t, req, http.StatusNoContent)

	unittest.AssertNotExistsBean(t, &project_model.Board{ID: 1})
}
