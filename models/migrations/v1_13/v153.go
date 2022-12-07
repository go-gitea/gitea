// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_13 //nolint

import (
	"xorm.io/xorm"
)

func AddTeamReviewRequestSupport(x *xorm.Engine) error {
	type Review struct {
		ReviewerTeamID int64 `xorm:"NOT NULL DEFAULT 0"`
	}

	type Comment struct {
		AssigneeTeamID int64 `xorm:"NOT NULL DEFAULT 0"`
	}

	if err := x.Sync2(new(Review)); err != nil {
		return err
	}

	return x.Sync2(new(Comment))
}
