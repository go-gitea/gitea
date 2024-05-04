// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models/db"
	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestPrivateRepoProject(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// not logged in user
	req := NewRequest(t, "GET", "/user31/-/projects")
	MakeRequest(t, req, http.StatusNotFound)

	sess := loginUser(t, "user1")
	req = NewRequest(t, "GET", "/user31/-/projects")
	sess.MakeRequest(t, req, http.StatusOK)
}

func TestMoveRepoProjectColumns(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	project1 := unittest.AssertExistsAndLoadBean(t, &project_model.Project{ID: 1})
	columns, err := project1.GetBoards(db.DefaultContext)
	assert.NoError(t, err)
	assert.Len(t, columns, 3)
	assert.EqualValues(t, 0, columns[0].Sorting)
	assert.EqualValues(t, 1, columns[1].Sorting)
	assert.EqualValues(t, 2, columns[2].Sorting)

	sess := loginUser(t, "user1")
	req := NewRequest(t, "GET", "/user2/repo1/projects/1")
	resp := sess.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	req = NewRequestWithJSON(t, "POST", "/user2/repo1/projects/1/move?_csrf="+htmlDoc.GetCSRF(), map[string]any{
		"columns": []map[string]any{
			{"columnID": columns[1].ID, "sorting": 0},
			{"columnID": columns[2].ID, "sorting": 1},
			{"columnID": columns[0].ID, "sorting": 2},
		},
	})
	sess.MakeRequest(t, req, http.StatusOK)

	columnsAfter, err := project1.GetBoards(db.DefaultContext)
	assert.NoError(t, err)
	assert.Len(t, columns, 3)
	assert.EqualValues(t, columns[1].ID, columnsAfter[0].ID)
	assert.EqualValues(t, columns[2].ID, columnsAfter[1].ID)
	assert.EqualValues(t, columns[0].ID, columnsAfter[2].ID)

	// update the sorting back
	req = NewRequestWithJSON(t, "POST", "/user2/repo1/projects/1/move?_csrf="+htmlDoc.GetCSRF(), map[string]any{
		"columns": []map[string]any{
			{"columnID": columns[0].ID, "sorting": 0},
			{"columnID": columns[1].ID, "sorting": 1},
			{"columnID": columns[2].ID, "sorting": 2},
		},
	})
	sess.MakeRequest(t, req, http.StatusOK)
}
