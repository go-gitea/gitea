// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import (
	"xorm.io/xorm"
)

func AddBackportVersion(x *xorm.Engine) error {
	type BackportVersion struct {
		ID      int64 `xorm:"pk autoincr"`
		Version int64
	}

	return x.Sync(new(BackportVersion))
}
