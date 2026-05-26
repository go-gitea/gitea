// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12

import "gitea.dev/models/db"

func AddBlockOnOutdatedBranch(x db.EngineMigration) error {
	type ProtectedBranch struct {
		BlockOnOutdatedBranch bool `xorm:"NOT NULL DEFAULT false"`
	}
	return x.Sync(new(ProtectedBranch))
}
