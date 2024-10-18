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

func TestAPIGetProjectColumn(t *testing.T) {
	expectedColumn := &api.Column{
		ID:    1,
		Title: "To Do",
	}

	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadAdmin, auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopeReadOrganization)

	t.Run("get column not found", func(t *testing.T) {
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/projects/columns/20")).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})
	t.Run("get column successfully", func(t *testing.T) {
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/projects/columns/1")).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var apiColumn *api.Column
		DecodeJSON(t, resp, &apiColumn)
		assert.Equal(t, expectedColumn.ID, apiColumn.ID)
		assert.Equal(t, expectedColumn.Title, apiColumn.Title)
	})
}

func TestAPIGetProjectColumns(t *testing.T) {

	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadAdmin, auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopeReadOrganization)

	expectedColumns := []*api.Column{
		{
			ID:    1,
			Title: "To Do",
		},
		{
			ID:    2,
			Title: "In Progress",
		},
		{
			ID:    3,
			Title: "Done",
		},
	}

	t.Run("failed to get columns project not found", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/projects/70/columns").AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})
	t.Run("get columns successfully", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/projects/1/columns").AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var apiColumns []*api.Column
		DecodeJSON(t, resp, &apiColumns)
		assert.Equal(t, len(expectedColumns), len(apiColumns))
		for i, expectedColumn := range expectedColumns {
			assert.Equal(t, expectedColumn.ID, apiColumns[i].ID)
			assert.Equal(t, expectedColumn.Title, apiColumns[i].Title)
		}
	})
}

func TestAPIAddColumnToProject(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteAdmin, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteOrganization)

	t.Run("add column to project successfully", func(t *testing.T) {
		req := NewRequestWithJSON(t, "POST", "/api/v1/projects/1/columns", &api.CreateProjectColumnOption{
			Title: "New Column",
		}).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusCreated)
		var apiColumn *api.Column
		DecodeJSON(t, resp, &apiColumn)
		assert.Equal(t, "New Column", apiColumn.Title)
	})
}

func TestAPIEditProjectColumn(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteAdmin, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteOrganization)

	t.Run("edit column successfully", func(t *testing.T) {
		req := NewRequestWithJSON(t, "PATCH", "/api/v1/projects/columns/1", &api.EditProjectColumnOption{
			Title: "Updated Column",
		}).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var apiColumn *api.Column
		DecodeJSON(t, resp, &apiColumn)
		assert.Equal(t, "Updated Column", apiColumn.Title)
	})
}

func TestAPIDeleteProjectColumn(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteAdmin, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteOrganization)

	t.Run("delete column successfully", func(t *testing.T) {
		req := NewRequest(t, "DELETE", "/api/v1/projects/columns/2").AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)
	})
}

func TestAPISetDefaultProjectColumn(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteAdmin, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteOrganization)

	t.Run("set default column successfully", func(t *testing.T) {
		req := NewRequestWithJSON(t, "PUT", "/api/v1/projects/columns/2/default", nil).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)
	})
}

func TestAPIMoveColumns(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteAdmin, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteOrganization)

	t.Run("move columns successfully", func(t *testing.T) {
		req := NewRequestWithJSON(t, "PATCH", "/api/v1/projects/1/columns/move", &api.MovedColumnsOption{
			Columns: []struct {
				ColumnID int64 `json:"columnID"`
				Sorting  int64 `json:"sorting"`
			}{
				{
					ColumnID: 3,
					Sorting:  1,
				},
				{
					ColumnID: 2,
					Sorting:  2,
				},
			},
		}).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var apiColumns []*api.Column
		DecodeJSON(t, resp, &apiColumns)
		assert.Equal(t, 2, len(apiColumns))
		assert.Equal(t, int64(3), apiColumns[0].ID)
		assert.Equal(t, int64(2), apiColumns[1].ID)
	})
}

func TestAPIMoveIssues(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteAdmin, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteOrganization)

	t.Run("move issues successfully", func(t *testing.T) {
		req := NewRequestWithJSON(t, "PATCH", "/api/v1/projects/1/columns/1/move", &api.MovedIssuesOption{
			Issues: []struct {
				IssueID int64 `json:"issueID"`
				Sorting int64 `json:"sorting"`
			}{
				{
					IssueID: 1,
					Sorting: 1,
				},
				{
					IssueID: 5,
					Sorting: 2,
				},
			},
		}).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var apiIssues []*api.Issue
		DecodeJSON(t, resp, &apiIssues)
		assert.Equal(t, 2, len(apiIssues))
		assert.Equal(t, int64(1), apiIssues[0].ID)
		assert.Equal(t, int64(2), apiIssues[1].ID)
	})
}
