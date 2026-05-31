// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20

import "gitea.dev/models/db"

func AddPinOrderToIssue(x db.EngineMigration) error {
	type Issue struct {
		PinOrder int `xorm:"DEFAULT 0"`
	}

	return x.Sync(new(Issue))
}
