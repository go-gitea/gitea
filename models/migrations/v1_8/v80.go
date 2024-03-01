// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_8 //nolint

import "xorm.io/xorm"

func AddIsLockedToIssues(x *xorm.Engine) error {
	// Issue see models/issue.go
	type Issue struct {
		ID       int64 `xorm:"pk autoincr"`
		IsLocked bool  `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(new(Issue))
}
