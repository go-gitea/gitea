// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func AddPullMergeIntentTable(_ context.Context, x base.EngineMigration) error {
	type PullMergeIntent struct {
		PullID   int64  `xorm:"pk"`
		CommitID string `xorm:"VARCHAR(64) NOT NULL"`
		MergerID int64  `xorm:"NOT NULL"`
		Auto     bool   `xorm:"NOT NULL"`
	}
	return x.Sync(new(PullMergeIntent))
}
