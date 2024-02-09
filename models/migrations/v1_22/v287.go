// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func CreateTablesForProjectBoardNotes(x *xorm.Engine) error {
	type ProjectBoardNote struct {
		ID          int64  `xorm:"pk autoincr"`
		Title       string `xorm:"TEXT NOT NULL"`
		Content     string `xorm:"LONGTEXT"`
		Sorting     int64  `xorm:"NOT NULL DEFAULT 0"`
		PinOrder    int64  `xorm:"NOT NULL DEFAULT 0"`
		MilestoneID int64  `xorm:"INDEX"`

		ProjectID int64 `xorm:"INDEX NOT NULL"`
		BoardID   int64 `xorm:"INDEX NOT NULL"`
		CreatorID int64 `xorm:"NOT NULL"`

		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	type ProjectBoardNoteLabel struct {
		ID                 int64 `xorm:"pk autoincr"`
		ProjectBoardNoteID int64 `xorm:"UNIQUE(s) NOT NULL"`
		LabelID            int64 `xorm:"UNIQUE(s) NOT NULL"`
	}

	err := x.Sync(new(ProjectBoardNote))
	if err != nil {
		return err
	}

	return x.Sync(new(ProjectBoardNoteLabel))
}
