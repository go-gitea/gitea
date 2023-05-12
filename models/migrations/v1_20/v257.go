// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import (
	"fmt"

	"xorm.io/xorm"
)

func AddMilestoneLabels(x *xorm.Engine) error {
	type MilestoneLabel struct {
		ID          int64 `xorm:"pk autoincr"`
		MilestoneID int64 `xorm:"UNIQUE(s)"`
		LabelID     int64 `xorm:"UNIQUE(s)"`
	}

	if err := x.Sync2(new(MilestoneLabel)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	return nil
}
