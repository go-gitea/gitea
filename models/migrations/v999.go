// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func creatUserBadgesTable(x *xorm.Engine) error {
	type Badge struct {
		ID          int64 `xorm:"pk autoincr"`
		Description string
		ImageURL    string
	}

	type UserBadge struct {
		ID      int64 `xorm:"pk autoincr"`
		BadgeID int64
		UserID  int64
	}

	if err := x.Sync2(new(Badge)); err != nil {
		return err
	}

	if err := x.Sync2(new(UserBadge)); err != nil {
		return err
	}
}
