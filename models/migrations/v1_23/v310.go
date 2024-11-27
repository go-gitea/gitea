// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import (
	"xorm.io/xorm"
)

func AddPriorityToProtectedBranch(x *xorm.Engine) error {
	type ProtectedBranch struct {
		Priority int64 `xorm:"NOT NULL DEFAULT 0"`
	}

	return x.Sync(new(ProtectedBranch))
}
