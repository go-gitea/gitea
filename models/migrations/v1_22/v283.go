// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"xorm.io/xorm"
)

func AddCombinedIndexToIssueUser(x *xorm.Engine) error {
	type IssueUser struct {
		UID     int64 `xorm:"INDEX unique(uid_to_issue)"` // User ID.
		IssueID int64 `xorm:"INDEX unique(uid_to_issue)"`
	}

	return x.Sync(&IssueUser{})
}
