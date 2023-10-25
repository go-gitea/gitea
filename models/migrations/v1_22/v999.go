// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"xorm.io/xorm"
)

func AddSizeLimitOnRepo(x *xorm.Engine) error {
	type Repository struct {
		ID        int64 `xorm:"pk autoincr"`
		SizeLimit int64 `xorm:"NOT NULL DEFAULT 0"`
	}

	return x.Sync2(new(Repository))
}
