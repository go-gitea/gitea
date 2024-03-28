// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/models/project"

	"github.com/stretchr/testify/assert"
)

func Test_CheckProjectColumnsConsistency(t *testing.T) {
	// Prepare and load the testing database
	x, deferable := base.PrepareTestEnv(t, 0, new(project.Project), new(project.Board))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	assert.NoError(t, CheckProjectColumnsConsistency(x))

	// check if default board was added
	var defaultBoard project.Board
	has, err := x.Where("project_id=? AND `default` = ?", 1, true).Get(&defaultBoard)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, int64(1), defaultBoard.ProjectID)
	assert.True(t, defaultBoard.Default)

	// check if multiple defaults, previous were removed and last will be kept
	expectDefaultBoard, err := project.GetBoard(db.DefaultContext, 2)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), expectDefaultBoard.ProjectID)
	assert.False(t, expectDefaultBoard.Default)

	expectNonDefaultBoard, err := project.GetBoard(db.DefaultContext, 3)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), expectNonDefaultBoard.ProjectID)
	assert.True(t, expectNonDefaultBoard.Default)
}
