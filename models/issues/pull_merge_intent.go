// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import "gitea.dev/models/db"

// PullMergeIntent identifies the commit prepared for a pull request merge before it is pushed.
type PullMergeIntent struct {
	PullID   int64  `xorm:"pk"`
	CommitID string `xorm:"VARCHAR(64) NOT NULL"`
	MergerID int64  `xorm:"NOT NULL"`
	Auto     bool   `xorm:"NOT NULL"`
}

func (*PullMergeIntent) TableName() string {
	return "pull_merge_intent"
}

func init() {
	db.RegisterModel(new(PullMergeIntent))
}
