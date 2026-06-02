// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21

import "gitea.dev/models/db"

// UpdateActionsRefIndex updates the index of actions ref field
func UpdateActionsRefIndex(x db.EngineMigration) error {
	type ActionRun struct {
		Ref string `xorm:"index"` // the commit/tag/… causing the run
	}
	return x.Sync(new(ActionRun))
}
