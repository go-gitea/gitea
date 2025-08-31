// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25

import "xorm.io/xorm"

func ExtendCommentTreePathLength(x *xorm.Engine) error {
	type Comment struct {
		TreePath string `xorm:"VARCHAR(1024)"`
	}

	return x.Sync(new(Comment))
}
