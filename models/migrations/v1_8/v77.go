// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_8 //nolint:revive // version underscore

import (
	"xorm.io/xorm"
)

func AddUserDefaultTheme(x *xorm.Engine) error {
	type User struct {
		Theme string `xorm:"VARCHAR(30) NOT NULL DEFAULT ''"`
	}

	return x.Sync(new(User))
}
