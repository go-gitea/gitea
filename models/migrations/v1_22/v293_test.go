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
	x, deferable := base.PrepareTestEnv(t, 0, new(project.Project), new(project.Column))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	assert.NoError(t, CheckProjectColumnsConsistency(x))

	// check if default column was added
	var defaultColumn project.Column
	has, err := x.Where("project_id=? AND `default` = ?", 1, true).Get(&defaultColumn)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.Equal(t, int64(1), defaultColumn.ProjectID)
	assert.True(t, defaultColumn.Default)

	// check if multiple defaults, previous were removed and last will be kept
	expectDefaultColumn, err := project.GetColumn(db.DefaultContext, 2)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), expectDefaultColumn.ProjectID)
	assert.False(t, expectDefaultColumn.Default)

	expectNonDefaultColumn, err := project.GetColumn(db.DefaultContext, 3)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), expectNonDefaultColumn.ProjectID)
	assert.True(t, expectNonDefaultColumn.Default)
}
