// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"xorm.io/xorm"
)

func AddCombinedIndexToIssueUser(x *xorm.Engine) error {
	type OldIssueUser struct {
		IssueID int64
		UID     int64
		Cnt     int64
	}

	var duplicatedIssueUsers []OldIssueUser
	if err := x.SQL("select * from (select issue_id, uid, count(1) as cnt from issue_user group by issue_id, uid) a where a.cnt > 1").
		Find(&duplicatedIssueUsers); err != nil {
		return err
	}
	for _, issueUser := range duplicatedIssueUsers {
		if _, err := x.Exec("delete from issue_user where id in (SELECT id FROM issue_user WHERE issue_id = ? and uid = ? limit ?)", issueUser.IssueID, issueUser.UID, issueUser.Cnt-1); err != nil {
			return err
		}
	}

	type IssueUser struct {
		UID     int64 `xorm:"INDEX unique(uid_to_issue)"` // User ID.
		IssueID int64 `xorm:"INDEX unique(uid_to_issue)"`
	}

	return x.Sync(&IssueUser{})
}
