// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint:revive // underscore in migration packages isn't a large issue

import (
	"xorm.io/xorm"
)

func AddIndexToIssueUserIssueID(x *xorm.Engine) error {
	type IssueUser struct {
		IssueID int64 `xorm:"INDEX"`
	}

	return x.Sync(new(IssueUser))
}
