// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

type repositoryWithLastPullSyncSuccessUnix struct {
	LastPullSyncSuccessUnix int64 `xorm:"INDEX"`
}

func (repositoryWithLastPullSyncSuccessUnix) TableName() string {
	return "repository"
}

func AddLastPullSyncSuccessUnixToRepository(x *xorm.Engine) error {
	if err := x.Sync(new(repositoryWithLastPullSyncSuccessUnix)); err != nil {
		return err
	}

	_, err := x.Exec("UPDATE repository SET last_pull_sync_success_unix = ?", int64(timeutil.TimeStampNow()))
	return err
}
