// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_13 //nolint:revive // underscore in migration packages isn't a large issue

import (
	"fmt"

	"xorm.io/xorm"
)

func AddKeepActivityPrivateUserColumn(x *xorm.Engine) error {
	type User struct {
		KeepActivityPrivate bool `xorm:"NOT NULL DEFAULT false"`
	}

	if err := x.Sync(new(User)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	return nil
}
