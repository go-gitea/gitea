// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func addCommentIDOnNotification(x *xorm.Engine) error {
	type Notification struct {
		ID        int64 `xorm:"pk autoincr"`
		CommentID int64
	}

	return x.Sync2(new(Notification))
}
