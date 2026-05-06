// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

type mirrorWithLastPullSyncSuccessUnix struct {
	LastPullSyncSuccessUnix int64 `xorm:"INDEX"`
}

func (mirrorWithLastPullSyncSuccessUnix) TableName() string {
	return "mirror"
}

func AddLastPullSyncSuccessUnixToMirror(x *xorm.Engine) error {
	if err := x.Sync(new(mirrorWithLastPullSyncSuccessUnix)); err != nil {
		return err
	}

	_, err := x.Exec("UPDATE mirror SET last_pull_sync_success_unix = ?", int64(timeutil.TimeStampNow()))
	return err
}
