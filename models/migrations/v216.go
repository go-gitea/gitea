// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/timeutil"
	"xorm.io/xorm"
)

func improveActionTableIndices(x *xorm.Engine) error {
	{
		type Action struct {
			ID          int64 `xorm:"pk autoincr"`
			UserID      int64 `xorm:"INDEX"` // Receiver user id.
			OpType      int
			ActUserID   int64 `xorm:"INDEX"` // Action user id.
			RepoID      int64 `xorm:"INDEX"`
			CommentID   int64 `xorm:"INDEX"`
			IsDeleted   bool  `xorm:"INDEX NOT NULL DEFAULT false"`
			RefName     string
			IsPrivate   bool               `xorm:"INDEX NOT NULL DEFAULT false"`
			Content     string             `xorm:"TEXT"`
			CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		}
		if err := x.Sync2(&Action{}); err != nil {
			return err
		}
		if err := x.DropIndexes(&Action{}); err != nil {
			return err
		}
	}
	{
		type Action struct {
			ID          int64 `xorm:"pk autoincr"`
			UserID      int64 `xorm:"INDEX(u_ua_and_r)"` // Receiver user id.
			OpType      int
			ActUserID   int64 `xorm:"INDEX(u_ua_and_r) INDEX(ua_and_r)"` // Action user id.
			RepoID      int64 `xorm:"INDEX(u_ua_and_r) INDEX(ua_and_r) INDEX(r)"`
			CommentID   int64 `xorm:"INDEX"`
			IsDeleted   bool  `xorm:"NOT NULL DEFAULT false"`
			RefName     string
			IsPrivate   bool               `xorm:"NOT NULL DEFAULT false"`
			Content     string             `xorm:"TEXT"`
			CreatedUnix timeutil.TimeStamp `xorm:"INDEX(u_ua_and_r) INDEX(ua_and_r) INDEX(r) created"`
		}
		return x.Sync2(&Action{})
	}
}
