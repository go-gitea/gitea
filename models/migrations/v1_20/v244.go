// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint:revive // underscore in migration packages isn't a large issue

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
