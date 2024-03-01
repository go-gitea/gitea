// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

type BoardNote struct {
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

// TableName xorm will read the table name from this method
func (BoardNote) TableName() string {
	return "project_board_note"
}

type BoardNoteLabel struct {
	ID          int64 `xorm:"pk autoincr"`
	BoardNoteID int64 `xorm:"UNIQUE(s) NOT NULL"`
	LabelID     int64 `xorm:"UNIQUE(s) NOT NULL"`
}

// TableName xorm will read the table name from this method
func (BoardNoteLabel) TableName() string {
	return "project_board_note_label"
}

func CreateTablesForBoardNotes(x *xorm.Engine) error {
	err := x.Sync(new(BoardNote))
	if err != nil {
		return err
	}

	return x.Sync(new(BoardNoteLabel))
}
