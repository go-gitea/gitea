// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"fmt"
	"math"
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestGetDefaultColumn(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	projectWithoutDefault, err := GetProjectByID(t.Context(), 5)
	assert.NoError(t, err)

	// check if default column was added
	column, err := projectWithoutDefault.MustDefaultColumn(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, int64(5), column.ProjectID)
	assert.Equal(t, "Done", column.Title)

	projectWithMultipleDefaults, err := GetProjectByID(t.Context(), 6)
	assert.NoError(t, err)

	// check if multiple defaults were removed
	column, err = projectWithMultipleDefaults.MustDefaultColumn(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, int64(6), column.ProjectID)
	assert.Equal(t, int64(9), column.ID) // there are 2 default columns in the test data, use the latest one

	// set 8 as default column
	assert.NoError(t, SetDefaultColumn(t.Context(), column.ProjectID, 8))

	// then 9 will become a non-default column
	column, err = GetColumn(t.Context(), 9)
	assert.NoError(t, err)
	assert.Equal(t, int64(6), column.ProjectID)
	assert.False(t, column.Default)
}

func Test_moveIssuesToAnotherColumn(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	column1 := unittest.AssertExistsAndLoadBean(t, &Column{ID: 1, ProjectID: 1})

	issues, err := column1.GetIssues(t.Context())
	assert.NoError(t, err)
	assert.Len(t, issues, 1)
	assert.EqualValues(t, 1, issues[0].ID)

	column2 := unittest.AssertExistsAndLoadBean(t, &Column{ID: 2, ProjectID: 1})
	issues, err = column2.GetIssues(t.Context())
	assert.NoError(t, err)
	assert.Len(t, issues, 1)
	assert.EqualValues(t, 3, issues[0].ID)

	err = moveIssuesToAnotherColumn(t.Context(), column1, column2)
	assert.NoError(t, err)

	issues, err = column1.GetIssues(t.Context())
	assert.NoError(t, err)
	assert.Empty(t, issues)

	issues, err = column2.GetIssues(t.Context())
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
	columns, err := GetColumns(t.Context(), project1.ID, db.ListOptionsAll)
	assert.NoError(t, err)
	assert.Len(t, columns, 3)
	assert.EqualValues(t, 0, columns[0].Sorting) // even if there is no default sorting, the code should also work
	assert.EqualValues(t, 0, columns[1].Sorting)
	assert.EqualValues(t, 0, columns[2].Sorting)

	err = MoveColumnsOnProject(t.Context(), project1, map[int64]int64{
		0: columns[1].ID,
		1: columns[2].ID,
		2: columns[0].ID,
	})
	assert.NoError(t, err)

	columnsAfter, err := GetColumns(t.Context(), project1.ID, db.ListOptionsAll)
	assert.NoError(t, err)
	assert.Len(t, columnsAfter, 3)
	assert.Equal(t, columns[1].ID, columnsAfter[0].ID)
	assert.Equal(t, columns[2].ID, columnsAfter[1].ID)
	assert.Equal(t, columns[0].ID, columnsAfter[2].ID)

	err = MoveColumnsOnProject(t.Context(), project1, map[int64]int64{200: columns[0].ID})
	assert.ErrorIs(t, err, util.ErrUnprocessableContent) // int8 column, 200 would wrap
}

func Test_NewColumn(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	project1 := unittest.AssertExistsAndLoadBean(t, &Project{ID: 1})
	columns, err := GetColumns(t.Context(), project1.ID, db.ListOptionsAll)
	assert.NoError(t, err)
	assert.Len(t, columns, 3)

	for i := range maxProjectColumns - 3 {
		err := NewColumn(t.Context(), &Column{
			Title:     fmt.Sprintf("column-%d", i+4),
			ProjectID: project1.ID,
		})
		assert.NoError(t, err)
	}
	err = NewColumn(t.Context(), &Column{
		Title:     "column-21",
		ProjectID: project1.ID,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum number of columns reached")
}

func Test_ColumnSorting(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	t.Run("appending an issue counts the legacy rows the default column renders", func(t *testing.T) {
		_, err := db.Exec(t.Context(), "UPDATE `project_issue` SET sorting=9 WHERE project_id=1 AND project_board_id=0")
		assert.NoError(t, err)

		defaultColumn, err := GetColumn(t.Context(), 1)
		assert.NoError(t, err)
		next, err := GetColumnIssueNextSorting(t.Context(), defaultColumn)
		assert.NoError(t, err)
		assert.EqualValues(t, 10, next)
	})

	t.Run("appending a column at the int8 maximum does not wrap to the front", func(t *testing.T) {
		_, err := db.Exec(t.Context(), "UPDATE `project_board` SET sorting=? WHERE id=3", math.MaxInt8)
		assert.NoError(t, err)

		appended := &Column{Title: "appended", ProjectID: 1}
		assert.NoError(t, NewColumn(t.Context(), appended))
		assert.EqualValues(t, math.MaxInt8, appended.Sorting)
	})
}
