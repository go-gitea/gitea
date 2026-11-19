// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"
)

func AddPullMergeQueueEntryTable(_ context.Context, x base.EngineMigration) error {
	type PullMergeQueueEntry struct {
		ID                int64  `xorm:"pk autoincr"`
		RepoID            int64  `xorm:"INDEX(branch) NOT NULL"`
		BaseBranch        string `xorm:"INDEX(branch) VARCHAR(255) NOT NULL"`
		PullID            int64  `xorm:"UNIQUE NOT NULL"`
		EnqueuerID        int64  `xorm:"NOT NULL"`
		Jump              bool   `xorm:"NOT NULL DEFAULT false"`
		State             string `xorm:"VARCHAR(20) NOT NULL"`
		HeadSHA           string `xorm:"VARCHAR(64) NOT NULL"`
		BaseSHA           string `xorm:"VARCHAR(64)"`
		GroupHeadSHA      string `xorm:"VARCHAR(64) INDEX"`
		GroupRef          string `xorm:"VARCHAR(255)"`
		ChecksStartedUnix timeutil.TimeStamp
		CreatedUnix       timeutil.TimeStamp `xorm:"created"`
	}
	return x.Sync(new(PullMergeQueueEntry))
}
