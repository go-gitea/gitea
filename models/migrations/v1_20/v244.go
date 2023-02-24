// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import (
	"xorm.io/xorm"
)

func AddNeedApprovalToActionRun(x *xorm.Engine) error {
	/*
		New index: TriggerUserID
		New fields: NeedApproval, ApprovedBy
	*/
	type ActionRun struct {
		TriggerUserID int64 `xorm:"index"`
		NeedApproval  bool  // may need approval if it's a fork pull request
		ApprovedBy    int64 `xorm:"index"` // who approved
	}

	return x.Sync(new(ActionRun))
}

// AddTimeEstimateColumnToIssueTable: add TimeEstimate column
func AddTimeEstimateColumnToIssueTable(x *xorm.Engine) error {
	type Issue struct {
		TimeEstimate int64
	}

	return x.Sync(new(Issue))
}

// AddColumnsToCommentTable: add TimeTracked column
func AddColumnsToCommentTable(x *xorm.Engine) error {
	type Comment struct {
		TimeTracked  int64
		TimeEstimate int64
	}

	return x.Sync(new(Comment))
}
