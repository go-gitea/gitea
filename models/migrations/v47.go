// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func addDeletedBranch(x *xorm.Engine) (err error) {
	// DeletedBranch contains the deleted branch information
	type DeletedBranch struct {
		ID          int64  `xorm:"pk autoincr"`
		RepoID      int64  `xorm:"UNIQUE(s) INDEX NOT NULL"`
		Name        string `xorm:"UNIQUE(s) NOT NULL"`
		Commit      string `xorm:"UNIQUE(s) NOT NULL"`
		DeletedByID int64  `xorm:"INDEX NOT NULL"`
		DeletedUnix int64  `xorm:"INDEX"`
	}

	if err = x.Sync2(new(DeletedBranch)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}

	return nil
}
