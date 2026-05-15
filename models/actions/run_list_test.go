// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestFindRunOptionsOrgAccessFilter(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	opts := FindRunOptions{
		OwnerID:      3,
		AccessUserID: 2,
	}

	runs, total, err := db.FindAndCount[ActionRun](t.Context(), opts)
	assert.NoError(t, err)
	if assert.Equal(t, int64(1), total) && assert.Len(t, runs, 1) {
		assert.Equal(t, int64(3), runs[0].RepoID)
	}
}

func TestFindRunJobOptionsOrgAccessFilter(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	opts := FindRunJobOptions{
		OwnerID:      3,
		AccessUserID: 2,
	}

	jobs, total, err := db.FindAndCount[ActionRunJob](t.Context(), opts)
	assert.NoError(t, err)
	if assert.Equal(t, int64(1), total) && assert.Len(t, jobs, 1) {
		assert.Equal(t, int64(3), jobs[0].RepoID)
	}
}
