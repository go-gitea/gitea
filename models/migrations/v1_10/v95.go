// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_10 //nolint

import "xorm.io/xorm"

func AddCrossReferenceColumns(x *xorm.Engine) error {
	// Comment see models/comment.go
	type Comment struct {
		RefRepoID    int64 `xorm:"index"`
		RefIssueID   int64 `xorm:"index"`
		RefCommentID int64 `xorm:"index"`
		RefAction    int64 `xorm:"SMALLINT"`
		RefIsPull    bool
	}

	return x.Sync(new(Comment))
}
