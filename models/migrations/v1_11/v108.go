// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_11 //nolint

import (
	"xorm.io/xorm"
)

func AddCommentIDOnNotification(x *xorm.Engine) error {
	type Notification struct {
		ID        int64 `xorm:"pk autoincr"`
		CommentID int64
	}

	return x.Sync(new(Notification))
}
