// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddArchivedUnixToRepository(x *xorm.Engine) error {
	type Repository struct {
		ArchivedUnix timeutil.TimeStamp `xorm:"DEFAULT 0"`
	}

	if err := x.Sync(new(Repository)); err != nil {
		return err
	}

	_, err := x.Exec("UPDATE repository SET archived_unix = updated_unix WHERE is_archived = ? AND archived_unix = 0", true)
	return err
}
