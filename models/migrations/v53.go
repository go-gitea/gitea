// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func addReactions(x *xorm.Engine) error {
	// Reaction see models/issue_reaction.go
	type Reaction struct {
		ID          int64  `xorm:"pk autoincr"`
		Type        string `xorm:"INDEX UNIQUE(s) NOT NULL"`
		IssueID     int64  `xorm:"INDEX UNIQUE(s) NOT NULL"`
		CommentID   int64  `xorm:"INDEX UNIQUE(s)"`
		UserID      int64  `xorm:"INDEX UNIQUE(s) NOT NULL"`
		CreatedUnix int64  `xorm:"INDEX created"`
	}

	if err := x.Sync2(new(Reaction)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	return nil
}
