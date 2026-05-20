// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestProjectColumns(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	t.Run("CountProjectColumns", testCountProjectColumns)
	t.Run("GetProjectColumns", testGetProjectColumns)
	t.Run("GetColumnsByIDs", testGetColumnsByIDs)
}

func testCountProjectColumns(t *testing.T) {
	project, err := GetProjectByID(t.Context(), 1)
	assert.NoError(t, err)

	count, err := CountProjectColumns(t.Context(), project.ID)
	assert.NoError(t, err)
	assert.EqualValues(t, 3, count)
}

func testGetProjectColumns(t *testing.T) {
	project, err := GetProjectByID(t.Context(), 1)
	assert.NoError(t, err)

	// Page 1, limit 2 — returns first 2 columns
	page1, err := GetProjectColumns(t.Context(), project.ID, db.ListOptions{Page: 1, PageSize: 2})
	assert.NoError(t, err)
	assert.Len(t, page1, 2)

	// Page 2, limit 2 — returns remaining column
	page2, err := GetProjectColumns(t.Context(), project.ID, db.ListOptions{Page: 2, PageSize: 2})
	assert.NoError(t, err)
	assert.Len(t, page2, 1)

	// Page 1 and page 2 together cover all columns with no overlap
	allIDs := make(map[int64]bool)
	for _, c := range append(page1, page2...) {
		assert.False(t, allIDs[c.ID], "duplicate column ID %d across pages", c.ID)
		allIDs[c.ID] = true
	}
	assert.Len(t, allIDs, 3)
}

func testGetColumnsByIDs(t *testing.T) {
	project, err := GetProjectByID(t.Context(), 1)
	assert.NoError(t, err)

	columns, err := GetColumnsByIDs(t.Context(), project.ID, []int64{1, 3, 4})
	assert.NoError(t, err)
	assert.Len(t, columns, 2)
	assert.ElementsMatch(t, []int64{1, 3}, []int64{columns[0].ID, columns[1].ID})

	empty, err := GetColumnsByIDs(t.Context(), project.ID, nil)
	assert.NoError(t, err)
	assert.Empty(t, empty)
}
