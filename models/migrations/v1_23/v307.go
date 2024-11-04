// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import "xorm.io/xorm"

func AddDefaultUnitToRepository(x *xorm.Engine) error {
	type Repository struct {
		DefaultUnit int `xorm:"NOT NULL DEFAULT 1"`
	}
	return x.Sync(new(Repository))
}
