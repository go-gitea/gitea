// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_14 //nolint:revive // underscore in migration packages isn't a large issue

import (
	"fmt"

	"xorm.io/xorm"
)

func AddSortingColToProjectBoard(x *xorm.Engine) error {
	type ProjectBoard struct {
		Sorting int8 `xorm:"NOT NULL DEFAULT 0"`
	}

	if err := x.Sync(new(ProjectBoard)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	return nil
}
