// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16

import (
	"fmt"

	"gitea.dev/modelmigration/base"
)

func AddColorColToProjectBoard(x base.EngineMigration) error {
	type ProjectBoard struct {
		Color string `xorm:"VARCHAR(7)"`
	}

	if err := x.Sync(new(ProjectBoard)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	return nil
}
