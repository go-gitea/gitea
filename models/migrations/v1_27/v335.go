// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"code.gitea.io/gitea/models/db"

	"xorm.io/xorm"
)

func AddIndexIssueDependencyDependencyID(x db.EngineMigration) error {
	type IssueDependency struct {
		DependencyID int64 `xorm:"INDEX"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(IssueDependency))
	return err
}
