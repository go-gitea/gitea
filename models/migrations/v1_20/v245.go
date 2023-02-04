// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddActionScheduleTable(x *xorm.Engine) error {
	type ActionSchedule struct {
		ID            int64
		Title         string
		Specs         []string
		EntryIDs      []int  `xorm:"entry_ids"`
		RepoID        int64  `xorm:"index"`
		OwnerID       int64  `xorm:"index"`
		WorkflowID    string `xorm:"index"` // the name of workflow file
		TriggerUserID int64
		Ref           string
		CommitSHA     string
		Event         string
		EventPayload  string `xorm:"LONGTEXT"`
		Content       []byte
		Created       timeutil.TimeStamp `xorm:"created"`
		Updated       timeutil.TimeStamp `xorm:"updated"`
	}

	return x.Sync(
		new(ActionSchedule),
	)
}
