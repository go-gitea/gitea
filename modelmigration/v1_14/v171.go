// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_14

import (
	"fmt"

	"gitea.dev/modelmigration/base"
)

func AddSortingColToProjectBoard(x base.EngineMigration) error {
	type ProjectBoard struct {
		Sorting int8 `xorm:"NOT NULL DEFAULT 0"`
	}

	if err := x.Sync(new(ProjectBoard)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	return nil
}
