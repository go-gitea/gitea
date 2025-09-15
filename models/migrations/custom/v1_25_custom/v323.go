// Copyright 2025 okTurtles Foundation. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25_custom

import (
	"xorm.io/xorm"
)

// AddSubjectToRepository adds a subject column to the repository table
// This migration was renumbered from 9001 to 323 for sequential compatibility
// Original migration ID: 9001 -> New migration ID: 323
func AddSubjectToRepository(x *xorm.Engine) error {
	type Repository struct {
		Subject string `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`
	}

	if err := x.Sync(new(Repository)); err != nil {
		return err
	}

	// Set subject to the current name for all existing repositories
	_, err := x.Exec("UPDATE repository SET subject = name WHERE subject = '' OR subject IS NULL")
	return err
}
