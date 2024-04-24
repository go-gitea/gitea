// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestProjectMovedIssuesForm_ToSortedIssueIDs(t *testing.T) {
	opts := &ProjectMovedIssuesForm{
		Issues: []ProjectMovedIssuesFormItem{
			{
				IssueID: 5,
				Sorting: 1,
			},
			{
				IssueID: 1,
				Sorting: 4,
			},
			{
				IssueID: 6,
				Sorting: 3,
			},
		},
	}

	ids, sorts := opts.ToSortedIssueIDs()

	assert.EqualValues(t, sorts, []int64{1, 3, 4})
	assert.EqualValues(t, ids, []int64{5, 6, 1})
}

func TestMoveIssuesOnProjectBoard(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	project := unittest.AssertExistsAndLoadBean(t, &project_model.Project{ID: 1})
	toBoard := unittest.AssertExistsAndLoadBean(t, &project_model.Board{ID: 2})

	list, err := LoadIssuesFromBoardList(db.DefaultContext, []*project_model.Board{toBoard})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, len(list[toBoard.ID]))
	assert.EqualValues(t, 3, list[toBoard.ID][0].ID)

	opts := &ProjectMovedIssuesForm{
		Issues: []ProjectMovedIssuesFormItem{
			{
				IssueID: 1,
				Sorting: 2,
			},
			{
				IssueID: 2,
				Sorting: 3,
			},
			{
				IssueID: 3,
				Sorting: 1,
			},
		},
	}
	assert.NoError(t, MoveIssuesOnProjectBoard(db.DefaultContext, doer, opts, project, toBoard))

	list, err = LoadIssuesFromBoardList(db.DefaultContext, []*project_model.Board{toBoard})
	assert.NoError(t, err)
	assert.EqualValues(t, 3, len(list[toBoard.ID]))

	assert.EqualValues(t, 3, list[toBoard.ID][0].ID)
	assert.EqualValues(t, 1, list[toBoard.ID][1].ID)
	assert.EqualValues(t, 2, list[toBoard.ID][2].ID)
}
