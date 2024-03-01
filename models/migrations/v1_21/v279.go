// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint

import (
	"xorm.io/xorm"
)

func AddIndexToActionUserID(x *xorm.Engine) error {
	type Action struct {
		UserID int64 `xorm:"INDEX"`
	}

	return x.Sync(new(Action))
}
