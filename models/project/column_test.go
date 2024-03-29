// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestGetDefaultcolumn(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	projectWithoutDefault, err := GetProjectByID(db.DefaultContext, 5)
	assert.NoError(t, err)

	// check if default column was added
	column, err := projectWithoutDefault.getDefaultColumn(db.DefaultContext)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), column.ProjectID)
	assert.Equal(t, "Uncategorized", column.Title)

	projectWithMultipleDefaults, err := GetProjectByID(db.DefaultContext, 6)
	assert.NoError(t, err)

	// check if multiple defaults were removed
	column, err = projectWithMultipleDefaults.getDefaultColumn(db.DefaultContext)
	assert.NoError(t, err)
	assert.Equal(t, int64(6), column.ProjectID)
	assert.Equal(t, int64(9), column.ID)

	// set 8 as default column
	assert.NoError(t, SetDefaultColumn(db.DefaultContext, column.ProjectID, 8))

	// then 9 will become a non-default column
	column, err = GetColumn(db.DefaultContext, 9)
	assert.NoError(t, err)
	assert.Equal(t, int64(6), column.ProjectID)
	assert.False(t, column.Default)
}
