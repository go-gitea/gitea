// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint:revive // underscore in migration packages isn't a large issue

import (
	"xorm.io/xorm"
)

func AddNewColumnForProject(x *xorm.Engine) error {
	type Project struct {
		OwnerID int64 `xorm:"INDEX"`
	}

	return x.Sync(new(Project))
}
