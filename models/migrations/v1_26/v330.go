// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"xorm.io/xorm"
)

// AddIndexIssueDependencyDependencyID adds an index on issue_dependency.dependency_id
// to speed up reverse lookups (finding issues blocked by a given issue).
func AddIndexIssueDependencyDependencyID(x *xorm.Engine) error {
	type IssueDependency struct {
		DependencyID int64 `xorm:"INDEX"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(IssueDependency))
	return err
}
