// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestBoardConsistencyCheck(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	assert.NoError(t, CheckBoardsConsistency(db.DefaultContext))

	projectWithoutDefault, err := GetProjectByID(db.DefaultContext, 5)
	assert.NoError(t, err)

	// check if default board was added
	_, err = projectWithoutDefault.getDefaultBoard(db.DefaultContext)
	assert.NoError(t, err)

	// check if multiple defaults were removed
	defaultBoard, err := GetBoard(db.DefaultContext, 8)
	assert.NoError(t, err)
	assert.True(t, defaultBoard.Default)

	nonDefaultBoard, err := GetBoard(db.DefaultContext, 9)
	assert.NoError(t, err)
	assert.False(t, nonDefaultBoard.Default)
}
