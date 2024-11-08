// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func FixMilestoneNoDueDate(x *xorm.Engine) error {
	type Milestone struct {
		DeadlineUnix timeutil.TimeStamp
	}
	// Wednesday, December 1, 9999 12:00:00 AM GMT+00:00
	_, err := x.Table("milestone").Where("deadline_unix > 253399622400").
		Cols("deadline_unix").
		Update(&Milestone{DeadlineUnix: 0})
	return err
}
