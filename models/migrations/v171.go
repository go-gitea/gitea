// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func addSortingColToProjectBoard(x *xorm.Engine) error {
	type ProjectBoard struct {
		Sorting int8 `xorm:"NOT NULL DEFAULT 0"`
	}

	if err := x.Sync2(new(ProjectBoard)); err != nil {
		return fmt.Errorf("Sync2: %w", err)
	}
	return nil
}
