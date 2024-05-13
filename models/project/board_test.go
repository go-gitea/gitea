// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"fmt"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestGetDefaultBoard(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	projectWithoutDefault, err := GetProjectByID(db.DefaultContext, 5)
	assert.NoError(t, err)

	// check if default board was added
	board, err := projectWithoutDefault.GetDefaultBoard(db.DefaultContext)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), board.ProjectID)
	assert.Equal(t, "Uncategorized", board.Title)

	projectWithMultipleDefaults, err := GetProjectByID(db.DefaultContext, 6)
	assert.NoError(t, err)

	// check if multiple defaults were removed
	board, err = projectWithMultipleDefaults.GetDefaultBoard(db.DefaultContext)
	assert.NoError(t, err)
	assert.Equal(t, int64(6), board.ProjectID)
	assert.Equal(t, int64(9), board.ID)

	// set 8 as default board
	assert.NoError(t, SetDefaultBoard(db.DefaultContext, board.ProjectID, 8))

	// then 9 will become a non-default board
	board, err = GetBoard(db.DefaultContext, 9)
	assert.NoError(t, err)
	assert.Equal(t, int64(6), board.ProjectID)
	assert.False(t, board.Default)
}

func Test_moveIssuesToAnotherColumn(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	column1 := unittest.AssertExistsAndLoadBean(t, &Board{ID: 1, ProjectID: 1})

	issues, err := column1.GetIssues(db.DefaultContext)
	assert.NoError(t, err)
	assert.Len(t, issues, 1)
	assert.EqualValues(t, 1, issues[0].ID)

	column2 := unittest.AssertExistsAndLoadBean(t, &Board{ID: 2, ProjectID: 1})
	issues, err = column2.GetIssues(db.DefaultContext)
	assert.NoError(t, err)
	assert.Len(t, issues, 1)
	assert.EqualValues(t, 3, issues[0].ID)

	err = column1.moveIssuesToAnotherColumn(db.DefaultContext, column2)
	assert.NoError(t, err)

	issues, err = column1.GetIssues(db.DefaultContext)
	assert.NoError(t, err)
	assert.Len(t, issues, 0)

	issues, err = column2.GetIssues(db.DefaultContext)
	assert.NoError(t, err)
	assert.Len(t, issues, 2)
	assert.EqualValues(t, 3, issues[0].ID)
	assert.EqualValues(t, 0, issues[0].Sorting)
	assert.EqualValues(t, 1, issues[1].ID)
	assert.EqualValues(t, 1, issues[1].Sorting)
}

func Test_MoveColumnsOnProject(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	project1 := unittest.AssertExistsAndLoadBean(t, &Project{ID: 1})
	columns, err := project1.GetBoards(db.DefaultContext)
	assert.NoError(t, err)
	assert.Len(t, columns, 3)
	assert.EqualValues(t, 0, columns[0].Sorting) // even if there is no default sorting, the code should also work
	assert.EqualValues(t, 0, columns[1].Sorting)
	assert.EqualValues(t, 0, columns[2].Sorting)

	err = MoveColumnsOnProject(db.DefaultContext, project1, map[int64]int64{
		0: columns[1].ID,
		1: columns[2].ID,
		2: columns[0].ID,
	})
	assert.NoError(t, err)

	columnsAfter, err := project1.GetBoards(db.DefaultContext)
	assert.NoError(t, err)
	assert.Len(t, columnsAfter, 3)
	assert.EqualValues(t, columns[1].ID, columnsAfter[0].ID)
	assert.EqualValues(t, columns[2].ID, columnsAfter[1].ID)
	assert.EqualValues(t, columns[0].ID, columnsAfter[2].ID)
}

func Test_NewBoard(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	project1 := unittest.AssertExistsAndLoadBean(t, &Project{ID: 1})
	columns, err := project1.GetBoards(db.DefaultContext)
	assert.NoError(t, err)
	assert.Len(t, columns, 3)

	for i := 0; i < maxProjectColumns-3; i++ {
		err := NewBoard(db.DefaultContext, &Board{
			Title:     fmt.Sprintf("board-%d", i+4),
			ProjectID: project1.ID,
		})
		assert.NoError(t, err)
	}
	err = NewBoard(db.DefaultContext, &Board{
		Title:     "board-21",
		ProjectID: project1.ID,
	})
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "maximum number of columns reached"))
}
