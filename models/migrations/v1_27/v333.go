// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"xorm.io/xorm"
)

func AddIndexIssueDependencyDependencyID(x *xorm.Engine) error {
	type IssueDependency struct {
		DependencyID int64 `xorm:"INDEX"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(IssueDependency))
	return err
}
