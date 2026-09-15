// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import "xorm.io/xorm"

func AddIssueReviewStates(x *xorm.Engine) error {
	type Issue struct {
		IsFirstReview  bool `xorm:"INDEX NOT NULL DEFAULT false"`
		IsSecondReview bool `xorm:"INDEX NOT NULL DEFAULT false"`
	}
	return x.Sync(new(Issue))
}
