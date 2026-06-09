// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_17

import (
	"gitea.dev/models/db"
	"gitea.dev/models/pull"
	"gitea.dev/modules/timeutil"
)

func AddReviewViewedFiles(x db.EngineMigration) error {
	type ReviewState struct {
		ID           int64                       `xorm:"pk autoincr"`
		UserID       int64                       `xorm:"NOT NULL UNIQUE(pull_commit_user)"`
		PullID       int64                       `xorm:"NOT NULL INDEX UNIQUE(pull_commit_user) DEFAULT 0"`
		CommitSHA    string                      `xorm:"NOT NULL VARCHAR(40) UNIQUE(pull_commit_user)"`
		UpdatedFiles map[string]pull.ViewedState `xorm:"NOT NULL LONGTEXT JSON"`
		UpdatedUnix  timeutil.TimeStamp          `xorm:"updated"`
	}

	return x.Sync(new(ReviewState))
}
