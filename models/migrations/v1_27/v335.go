// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"code.gitea.io/gitea/models/db"
)

// AddBlockOnCodeownerReviews adds block on codeowner reviews branch protection
func AddBlockOnCodeownerReviews(x db.EngineMigration) error {
	type ProtectedBranch struct {
		BlockOnCodeownerReviews bool `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(new(ProtectedBranch))
}
