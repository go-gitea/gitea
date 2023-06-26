// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint

import (
	"xorm.io/xorm"
)

func AddStartLineAndIsMultiLineToComment(x *xorm.Engine) error {
	type Comment struct {
		StartLine   int64 // - previous line / + proposed line
		IsMultiLine bool  `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(new(Comment))
}
