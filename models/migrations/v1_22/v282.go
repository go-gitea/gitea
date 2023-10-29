// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddProjectAutomationTable(x *xorm.Engine) error {
	type (
		AutomationTriggerType      uint8
		AutomationActionType       uint8
		AutomationActionTargetType uint8
	)

	type ProjectAutomation struct {
		ID             int64                      `xorm:"pk autoincr"`
		Enabled        bool                       `xorm:"INDEX NOT NULL DEFAULT true"`
		ProjectID      int64                      `xorm:"INDEX NOT NULL"`
		ProjectBoardID int64                      `xorm:"INDEX NOT NULL"`
		TriggerType    AutomationTriggerType      `xorm:"INDEX NOT NULL"`
		TriggerData    int64                      `xorm:"INDEX NOT NULL DEFAULT 0"`
		ActionType     AutomationActionType       `xorm:"NOT NULL DEFAULT 0"`
		ActionData     int64                      `xorm:"NOT NULL DEFAULT 0"`
		ActionTarget   AutomationActionTargetType `xorm:"NOT NULL DEFAULT 0"`
		Sorting        int64                      `xorm:"NOT NULL DEFAULT 0"`

		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	return x.Sync(new(ProjectAutomation))
}
