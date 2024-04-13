// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
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
	board, err := projectWithoutDefault.getDefaultBoard(db.DefaultContext)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), board.ProjectID)
	assert.Equal(t, "Uncategorized", board.Title)

	projectWithMultipleDefaults, err := GetProjectByID(db.DefaultContext, 6)
	assert.NoError(t, err)

	// check if multiple defaults were removed
	board, err = projectWithMultipleDefaults.getDefaultBoard(db.DefaultContext)
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
