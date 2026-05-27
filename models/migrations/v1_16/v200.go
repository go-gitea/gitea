// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16

import (
	"fmt"

	"gitea.dev/models/db"
)

func AddTableAppState(x db.EngineMigration) error {
	type AppState struct {
		ID       string `xorm:"pk varchar(200)"`
		Revision int64
		Content  string `xorm:"LONGTEXT"`
	}
	if err := x.Sync(new(AppState)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	return nil
}
