// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"
)

func AddPullMergeQueueEntryTable(_ctx context.Context, engine base.EngineMigration) error {
	type PullMergeQueueEntry struct {
		ID                int64              `xorm:"pk autoincr"`
		RepoID            int64              `xorm:"INDEX(branch) NOT NULL"`
		BaseBranch        string             `xorm:"INDEX(branch) VARCHAR(255) NOT NULL"`
		PullID            int64              `xorm:"UNIQUE NOT NULL"`
		EnqueuerID        int64              `xorm:"NOT NULL"`
		JumpUnix          timeutil.TimeStamp `xorm:"NOT NULL DEFAULT 0"`
		State             string             `xorm:"VARCHAR(20) NOT NULL"`
		HeadSHA           string             `xorm:"VARCHAR(64) NOT NULL"`
		BaseSHA           string             `xorm:"VARCHAR(64)"`
		GroupHeadSHA      string             `xorm:"VARCHAR(64) INDEX"`
		GroupRef          string             `xorm:"VARCHAR(512)"`
		ChecksStartedUnix timeutil.TimeStamp
		CreatedUnix       timeutil.TimeStamp `xorm:"created"`
	}
	return engine.Sync(new(PullMergeQueueEntry))
}
