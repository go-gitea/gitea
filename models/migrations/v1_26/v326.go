// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import "xorm.io/xorm"

type UserHeatmapCommit struct {
	ID              int64  `xorm:"pk autoincr"`
	UserID          int64  `xorm:"INDEX"`
	RepoID          int64  `xorm:"INDEX"`
	CommitSha1      string `xorm:"VARCHAR(64)"`
	CommitTimestamp int64  `xorm:"INDEX"`
}

func CreateUserHeatmapCommitTable(x *xorm.Engine) error {
	return x.Sync(new(UserHeatmapCommit))
}
