// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func addCommentIDToAction(x *xorm.Engine) error {
	// Action see models/action.go
	type Action struct {
		CommentID int64 `xorm:"INDEX"`
		IsDeleted bool  `xorm:"INDEX NOT NULL DEFAULT false"`
	}

	if err := x.Sync2(new(Action)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}

	return nil
}
