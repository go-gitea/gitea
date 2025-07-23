// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func Test_FixReviewStateUpdatedFilesColumn(t *testing.T) {
	if setting.Database.Type == "sqlite3" {
		t.Skip("SQLite does not support modify column type")
	}

	type ReviewState struct {
		ID           int64              `xorm:"pk autoincr"`
		UserID       int64              `xorm:"NOT NULL UNIQUE(pull_commit_user)"`
		PullID       int64              `xorm:"NOT NULL INDEX UNIQUE(pull_commit_user) DEFAULT 0"` // Which PR was the review on?
		CommitSHA    string             `xorm:"NOT NULL VARCHAR(64) UNIQUE(pull_commit_user)"`     // Which commit was the head commit for the review?
		UpdatedFiles map[string]int     `xorm:"NOT NULL TEXT JSON"`                                // Stores for each of the changed files of a PR whether they have been viewed, changed since last viewed, or not viewed
		UpdatedUnix  timeutil.TimeStamp `xorm:"updated"`                                           // Is an accurate indicator of the order of commits as we do not expect it to be possible to make reviews on previous commits
	}

	// Prepare and load the testing database
	x, deferable := base.PrepareTestEnv(t, 0, new(ReviewState))
	defer deferable()

	assert.NoError(t, FixReviewStateUpdatedFilesColumn(x))

	tableInfo, err := x.TableInfo(&ReviewState{})
	assert.NoError(t, err)
	assert.NotNil(t, tableInfo)
	column := tableInfo.GetColumn("updated_files")
	assert.NotNil(t, column)
	assert.Equal(t, "LONGTEXT", column.SQLType.Name)
}
