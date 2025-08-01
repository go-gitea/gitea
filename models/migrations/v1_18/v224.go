// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_18

import (
	"xorm.io/xorm"
)

func CreateUserBadgesTable(x *xorm.Engine) error {
	type Badge struct {
		ID          int64 `xorm:"pk autoincr"`
		Description string
		ImageURL    string
	}

	type userBadge struct {
		ID      int64 `xorm:"pk autoincr"`
		BadgeID int64
		UserID  int64 `xorm:"INDEX"`
	}

	if err := x.Sync(new(Badge)); err != nil {
		return err
	}
	return x.Sync(new(userBadge))
}
