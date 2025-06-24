// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12 //nolint:revive // underscore in migration packages isn't a large issue

import "xorm.io/xorm"

func AddIsRestricted(x *xorm.Engine) error {
	// User see models/user.go
	type User struct {
		ID           int64 `xorm:"pk autoincr"`
		IsRestricted bool  `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(new(User))
}
