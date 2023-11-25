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

func TestAPICreateProjectBoard(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	token := getUserToken(
		t,
		"user2",
		auth_model.AccessTokenScopeWriteIssue,
	)

	link, _ := url.Parse(fmt.Sprintf("/api/v1/projects/%d/boards?token=%s", 1, token))

	req := NewRequestWithJSON(t, "POST", link.String(), &api.NewProjectBoardPayload{
		Title: "Unused",
	})
	resp := MakeRequest(t, req, http.StatusCreated)

	var apiProjectBoard *api.ProjectBoard
	DecodeJSON(t, resp, &apiProjectBoard)

	assert.Equal(t, apiProjectBoard.Title, "Unused")
	unittest.AssertExistsAndLoadBean(t, &project_model.Board{ID: apiProjectBoard.ID})
}

func TestAPIListProjectBoards(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	token := getUserToken(
		t,
		"user2",
		auth_model.AccessTokenScopeWriteIssue,
	)

	link, _ := url.Parse(fmt.Sprintf("/api/v1/projects/%d/boards?token=%s", 1, token))

	req := NewRequest(t, "GET", link.String())
	resp := MakeRequest(t, req, http.StatusOK)

	var apiProjectBoards []*api.ProjectBoard
	DecodeJSON(t, resp, &apiProjectBoards)

	assert.Len(t, apiProjectBoards, 4)
}

func TestAPIGetProjectBoard(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	token := getUserToken(
		t,
		"user2",
		auth_model.AccessTokenScopeReadIssue,
	)

	link, _ := url.Parse(fmt.Sprintf("/api/v1/projects/boards/%d?token=%s", 1, token))

	req := NewRequest(t, "GET", link.String())
	resp := MakeRequest(t, req, http.StatusOK)

	var apiProjectBoard *api.ProjectBoard
	DecodeJSON(t, resp, &apiProjectBoard)

	assert.Equal(t, apiProjectBoard.Title, "To Do")
}

func TestAPIUpdateProjectBoard(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	token := getUserToken(
		t,
		"user2",
		auth_model.AccessTokenScopeWriteIssue,
	)

	link, _ := url.Parse(fmt.Sprintf("/api/v1/projects/boards/%d?token=%s", 1, token))

	req := NewRequestWithJSON(t, "PATCH", link.String(), &api.UpdateProjectBoardPayload{
		Title: "Unused",
	})
	resp := MakeRequest(t, req, http.StatusOK)

	var apiProjectBoard *api.ProjectBoard
	DecodeJSON(t, resp, &apiProjectBoard)

	assert.Equal(t, apiProjectBoard.Title, "Unused")
	dbboard := &project_model.Board{ID: apiProjectBoard.ID}
	unittest.AssertExistsAndLoadBean(t, dbboard)
	assert.Equal(t, dbboard.Title, "Unused")
}

func TestAPIDeleteProjectBoard(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	token := getUserToken(
		t,
		"user2",
		auth_model.AccessTokenScopeWriteIssue,
	)

	link, _ := url.Parse(fmt.Sprintf("/api/v1/projects/boards/%d?token=%s", 1, token))

	req := NewRequest(t, "DELETE", link.String())
	MakeRequest(t, req, http.StatusNoContent)

	unittest.AssertNotExistsBean(t, &project_model.Board{ID: 1})
}
