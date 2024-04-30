// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import "xorm.io/xorm"

func AddCommitStatusSummary(x *xorm.Engine) error {
	type CommitStatusSummary struct {
		ID     int64  `xorm:"pk autoincr"`
		RepoID int64  `xorm:"INDEX UNIQUE(repo_id_sha)"`
		SHA    string `xorm:"VARCHAR(64) NOT NULL INDEX UNIQUE(repo_id_sha)"`
		State  string `xorm:"VARCHAR(7) NOT NULL"`
	}
	// there is no migrations because if there is no data on this table, it will fall back to get data
	// from commit status
	return x.Sync2(new(CommitStatusSummary))
}
