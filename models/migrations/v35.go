// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"github.com/go-xorm/xorm"
)

func addCommentIDToAction(x *xorm.Engine) error {
	// Action see models/action.go
	type Action struct {
		CommentID      int64 `xorm:"INDEX"`
		CommentDeleted bool  `xorm:"default 0 not null"`
	}

	if err := x.Sync2(new(Action)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}

	x.Where("comment_id = 0").Update(&Action{
		CommentDeleted: false,
	})

	return nil
}
