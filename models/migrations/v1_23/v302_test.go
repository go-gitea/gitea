// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func Test_AddIndexToActionTaskStoppedLogExpired(t *testing.T) {
	type ActionTask struct {
		ID       int64
		JobID    int64
		Attempt  int64
		RunnerID int64              `xorm:"index"`
		Status   int                `xorm:"index"`
		Started  timeutil.TimeStamp `xorm:"index"`
		Stopped  timeutil.TimeStamp `xorm:"index(stopped_log_expired)"`

		RepoID            int64  `xorm:"index"`
		OwnerID           int64  `xorm:"index"`
		CommitSHA         string `xorm:"index"`
		IsForkPullRequest bool

		Token          string `xorm:"-"`
		TokenHash      string `xorm:"UNIQUE"` // sha256 of token
		TokenSalt      string
		TokenLastEight string `xorm:"index token_last_eight"`

		LogFilename  string  // file name of log
		LogInStorage bool    // read log from database or from storage
		LogLength    int64   // lines count
		LogSize      int64   // blob size
		LogIndexes   []int64 `xorm:"LONGBLOB"`                   // line number to offset
		LogExpired   bool    `xorm:"index(stopped_log_expired)"` // files that are too old will be deleted

		Created timeutil.TimeStamp `xorm:"created"`
		Updated timeutil.TimeStamp `xorm:"updated index"`
	}

	// Prepare and load the testing database
	x, deferable := base.PrepareTestEnv(t, 0, new(ActionTask))
	defer deferable()

	assert.NoError(t, AddIndexToActionTaskStoppedLogExpired(x))
}
