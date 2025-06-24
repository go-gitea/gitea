// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16 //nolint:revive // underscore in migration packages isn't a large issue

import (
	"fmt"

	"xorm.io/xorm"
)

func AddColorColToProjectBoard(x *xorm.Engine) error {
	type ProjectBoard struct {
		Color string `xorm:"VARCHAR(7)"`
	}

	if err := x.Sync(new(ProjectBoard)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	return nil
}
