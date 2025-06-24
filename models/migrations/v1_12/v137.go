// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12 //nolint:revive // underscore in migration packages isn't a large issue

import (
	"xorm.io/xorm"
)

func AddBlockOnOutdatedBranch(x *xorm.Engine) error {
	type ProtectedBranch struct {
		BlockOnOutdatedBranch bool `xorm:"NOT NULL DEFAULT false"`
	}
	return x.Sync(new(ProtectedBranch))
}
