// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models/db"
	project_model "code.gitea.io/gitea/models/project"
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

	project1 := project_model.Project{
		Title:     "new created project",
		RepoID:    1,
		Type:      project_model.TypeRepository,
		BoardType: project_model.BoardTypeNone,
	}
	err := project_model.NewProject(db.DefaultContext, &project1)
	assert.NoError(t, err)

	for i := 0; i < 3; i++ {
		err = project_model.NewBoard(db.DefaultContext, &project_model.Board{
			Title:     fmt.Sprintf("column %d", i+1),
			ProjectID: project1.ID,
		})
		assert.NoError(t, err)
	}

	columns, err := project1.GetBoards(db.DefaultContext)
	assert.NoError(t, err)
	assert.Len(t, columns, 3)
	assert.EqualValues(t, 0, columns[0].Sorting)
	assert.EqualValues(t, 1, columns[1].Sorting)
	assert.EqualValues(t, 2, columns[2].Sorting)

	sess := loginUser(t, "user2")
	req := NewRequest(t, "GET", fmt.Sprintf("/user2/repo1/projects/%d", project1.ID))
	resp := sess.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/user2/repo1/projects/%d/move?_csrf="+htmlDoc.GetCSRF(), project1.ID), map[string]any{
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
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/user2/repo1/projects/%d/move?_csrf="+htmlDoc.GetCSRF(), project1.ID), map[string]any{
		"columns": []map[string]any{
			{"columnID": columns[0].ID, "sorting": 0},
			{"columnID": columns[1].ID, "sorting": 1},
			{"columnID": columns[2].ID, "sorting": 2},
		},
	})
	sess.MakeRequest(t, req, http.StatusOK)

	assert.NoError(t, project_model.DeleteProjectByID(db.DefaultContext, project1.ID))
}
