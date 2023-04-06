// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_13 //nolint

import (
	"fmt"

	"xorm.io/xorm"
)

func AddKeepActivityPrivateUserColumn(x *xorm.Engine) error {
	type User struct {
		KeepActivityPrivate bool `xorm:"NOT NULL DEFAULT false"`
	}

	if err := x.Sync2(new(User)); err != nil {
		return fmt.Errorf("Sync2: %w", err)
	}
	return nil
}
