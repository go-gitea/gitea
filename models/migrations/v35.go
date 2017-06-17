// Copyright 2017 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"github.com/go-xorm/xorm"
	"time"
)

func addCommentIdToAction(x *xorm.Engine) error {
	// Action see models/action.go
	type Action struct {
		ID          int64 `xorm:"pk autoincr"`
		UserID      int64 `xorm:"INDEX"` // Receiver user id.
		ActUserID   int64       `xorm:"INDEX"` // Action user id.
		RepoID      int64       `xorm:"INDEX"`
		CommentID   int64	`xorm:"INDEX"`
		RefName     string
		IsPrivate   bool      `xorm:"INDEX NOT NULL DEFAULT false"`
		Content     string    `xorm:"TEXT"`
		Created     time.Time `xorm:"-"`
		CreatedUnix int64     `xorm:"INDEX"`
	}

	var actions []*Action
	if err := x.Find(&actions); err != nil {
		return fmt.Errorf("Find: %v", err)
	}

	if err := x.DropTables(new(Action)); err != nil {
		return fmt.Errorf("DropTables: %v", err)
	}

	if err := x.Sync2(new(Action)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}

	if _, err := x.Insert(actions); err != nil {
		return fmt.Errorf("Insert: %v", err)
	}
	return nil
}
