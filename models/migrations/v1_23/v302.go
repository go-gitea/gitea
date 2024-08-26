// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddIndexToActionTaskStoppedLogExpired(x *xorm.Engine) error {
	type ActionTask struct {
		Stopped    timeutil.TimeStamp `xorm:"index(stopped_log_expired)"`
		LogExpired bool               `xorm:"index(stopped_log_expired)"`
	}
	return x.Sync(new(ActionTask))
}
