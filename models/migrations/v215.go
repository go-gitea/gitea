// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/models/pull"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func addReviewViewedFiles(x *xorm.Engine) error {
	type ReviewState struct {
		ID           int64                       `xorm:"pk autoincr"`
		UserID       int64                       `xorm:"NOT NULL UNIQUE(pull_commit_user)"`
		PullID       int64                       `xorm:"NOT NULL INDEX UNIQUE(pull_commit_user) DEFAULT 0"`
		CommitSHA    string                      `xorm:"NOT NULL VARCHAR(40) UNIQUE(pull_commit_user)"`
		UpdatedFiles map[string]pull.ViewedState `xorm:"NOT NULL LONGTEXT JSON"`
		UpdatedUnix  timeutil.TimeStamp          `xorm:"updated"`
	}

	return x.Sync2(new(ReviewState))
}
