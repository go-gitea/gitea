// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v1_18 // nolint

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

	if err := x.Sync2(new(Badge)); err != nil {
		return err
	}
	return x.Sync2(new(userBadge))
}
