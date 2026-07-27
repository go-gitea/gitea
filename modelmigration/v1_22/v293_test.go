// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func Test_CheckProjectColumnsConsistency(t *testing.T) {
	type Project struct {
		ID           int64
		Title        string
		Description  string
		OwnerID      int64
		RepoID       int64
		CreatorID    int64
		IsClosed     bool
		TemplateType uint8
		BoardType    uint8
		Type         uint8

		CreatedUnix    timeutil.TimeStamp
		UpdatedUnix    timeutil.TimeStamp
		ClosedDateUnix timeutil.TimeStamp
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(Project), new(ProjectBoardV293))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	assert.NoError(t, CheckProjectColumnsConsistency(x))

	// check if default column was added
	var defaultColumn ProjectBoardV293
	has, err := x.Where("project_id=? AND `default` = ?", 1, true).Get(&defaultColumn)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, int64(1), defaultColumn.ProjectID)
	assert.True(t, defaultColumn.Default)

	// check if multiple defaults, previous were removed and last will be kept
	var expectDefaultColumn ProjectBoardV293
	has, err = x.ID(2).Get(&expectDefaultColumn)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, int64(2), expectDefaultColumn.ProjectID)
	assert.False(t, expectDefaultColumn.Default)

	var expectNonDefaultColumn ProjectBoardV293
	has, err = x.ID(3).Get(&expectNonDefaultColumn)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, int64(2), expectNonDefaultColumn.ProjectID)
	assert.True(t, expectNonDefaultColumn.Default)
}
