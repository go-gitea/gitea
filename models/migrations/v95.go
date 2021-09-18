// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import "xorm.io/xorm"

func addCrossReferenceColumns(x *xorm.Engine) error {
	// Comment see models/comment.go
	type Comment struct {
		RefRepoID    int64 `xorm:"index"`
		RefIssueID   int64 `xorm:"index"`
		RefCommentID int64 `xorm:"index"`
		RefAction    int64 `xorm:"SMALLINT"`
		RefIsPull    bool
	}

	return x.Sync2(new(Comment))
}
