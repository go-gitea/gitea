// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12

import (
	"fmt"

	"gitea.dev/models/db"
)

func AddOrgIDLabelColumn(x db.EngineMigration) error {
	type Label struct {
		OrgID int64 `xorm:"INDEX"`
	}

	if err := x.Sync(new(Label)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	return nil
}
