// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_8 //nolint

import (
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func AddCanCloseIssuesViaCommitInAnyBranch(x *xorm.Engine) error {
	type Repository struct {
		ID                              int64 `xorm:"pk autoincr"`
		CloseIssuesViaCommitInAnyBranch bool  `xorm:"NOT NULL DEFAULT false"`
	}

	if err := x.Sync(new(Repository)); err != nil {
		return err
	}

	_, err := x.Exec("UPDATE repository SET close_issues_via_commit_in_any_branch = ?",
		setting.Repository.DefaultCloseIssuesViaCommitsInAnyBranch)
	return err
}
