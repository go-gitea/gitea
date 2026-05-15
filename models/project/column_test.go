// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"fmt"
	"testing"

	"code.gitea.io/gitea/models/unittest"

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

func Test_SetColumnSortings_MovesAll(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	project1 := unittest.AssertExistsAndLoadBean(t, &Project{ID: 1})
	columns, err := project1.GetColumns(t.Context())
	assert.NoError(t, err)
	assert.Len(t, columns, 3)
	assert.EqualValues(t, 0, columns[0].Sorting)
	assert.EqualValues(t, 1, columns[1].Sorting)
	assert.EqualValues(t, 2, columns[2].Sorting)

	err = SetColumnSortings(t.Context(), project1.ID, map[int64]int64{
		columns[1].ID: 0,
		columns[2].ID: 1,
		columns[0].ID: 2,
	})
	assert.NoError(t, err)

	columnsAfter, err := project1.GetColumns(t.Context())
	assert.NoError(t, err)
	assert.Len(t, columnsAfter, 3)
	assert.Equal(t, columns[1].ID, columnsAfter[0].ID)
	assert.Equal(t, columns[2].ID, columnsAfter[1].ID)
	assert.Equal(t, columns[0].ID, columnsAfter[2].ID)
}

func Test_NewColumn(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	project1 := unittest.AssertExistsAndLoadBean(t, &Project{ID: 1})
	columns, err := project1.GetColumns(t.Context())
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
