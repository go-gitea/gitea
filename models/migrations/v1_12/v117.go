// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12 //nolint

import (
	"xorm.io/xorm"
)

func AddBlockOnRejectedReviews(x *xorm.Engine) error {
	type ProtectedBranch struct {
		BlockOnRejectedReviews bool `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(new(ProtectedBranch))
}
