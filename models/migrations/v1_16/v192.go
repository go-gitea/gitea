// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16 //nolint:revive // underscore in migration packages isn't a large issue

import (
	"code.gitea.io/gitea/models/migrations/base"

	"xorm.io/xorm"
)

func RecreateIssueResourceIndexTable(x *xorm.Engine) error {
	type IssueIndex struct {
		GroupID  int64 `xorm:"pk"`
		MaxIndex int64 `xorm:"index"`
	}

	return base.RecreateTables(new(IssueIndex))(x)
}
