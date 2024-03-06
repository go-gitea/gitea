// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint
import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddArchivedUnixColumInLabelTable(x *xorm.Engine) error {
	type Label struct {
		ArchivedUnix timeutil.TimeStamp `xorm:"DEFAULT NULL"`
	}
	return x.Sync(new(Label))
}
