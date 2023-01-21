// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19 //nolint

import (
	"xorm.io/xorm"
)

func AddOriginalAssignee(x *xorm.Engine) error {
	type IssueAssignees struct {
		OriginalAssignee   string
		OriginalAssigneeID int64 `xorm:"index"`
	}
	return x.Sync(new(IssueAssignees))
}
