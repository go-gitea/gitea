// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"xorm.io/xorm"
)

// AddBlockOnCodeownerReviews adds block on codeowner reviews branch protection
func AddBlockOnCodeownerReviews(x *xorm.Engine) error {
	type ProtectedBranch struct {
		BlockOnCodeownerReviews bool `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(new(ProtectedBranch))
}
