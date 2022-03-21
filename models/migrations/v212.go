// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/models/pulls"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func addPRReviewedFiles(x *xorm.Engine) error {
	type PRReview struct {
		ID          int64                        `xorm:"pk autoincr"`
		UserID      int64                        `xorm:"NOT NULL UNIQUE(pull_commit_user)"`
		ViewedFiles map[string]pulls.ViewedState `xorm:"TEXT JSON"`
		CommitSHA   string                       `xorm:"NOT NULL UNIQUE(pull_commit_user)"`
		PullID      int64                        `xorm:"NOT NULL UNIQUE(pull_commit_user)"`
		UpdatedUnix timeutil.TimeStamp           `xorm:"updated"`
	}

	return x.Sync2(new(PRReview))
}
