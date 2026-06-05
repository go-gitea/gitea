// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21

import (
	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"
)

func AddArchivedUnixColumInLabelTable(x db.EngineMigration) error {
	type Label struct {
		ArchivedUnix timeutil.TimeStamp `xorm:"DEFAULT NULL"`
	}
	return x.Sync(new(Label))
}
