// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_14 //nolint

import (
	"fmt"

	"xorm.io/xorm"
)

func AddUserRedirect(x *xorm.Engine) (err error) {
	type UserRedirect struct {
		ID             int64  `xorm:"pk autoincr"`
		LowerName      string `xorm:"UNIQUE(s) INDEX NOT NULL"`
		RedirectUserID int64
	}

	if err := x.Sync2(new(UserRedirect)); err != nil {
		return fmt.Errorf("Sync2: %w", err)
	}
	return nil
}
