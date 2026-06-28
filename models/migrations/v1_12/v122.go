// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12

import "gitea.dev/models/db"

func AddRequireSignedCommits(x db.EngineMigration) error {
	type ProtectedBranch struct {
		RequireSignedCommits bool `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(new(ProtectedBranch))
}
