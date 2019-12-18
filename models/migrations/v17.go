// Copyright 2016 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"
	"time"

	"xorm.io/xorm"
)

func setProtectedBranchUpdatedWithCreated(x *xorm.Engine) (err error) {
	type ProtectedBranch struct {
		ID          int64  `xorm:"pk autoincr"`
		RepoID      int64  `xorm:"UNIQUE(s)"`
		BranchName  string `xorm:"UNIQUE(s)"`
		CanPush     bool
		Created     time.Time `xorm:"-"`
		CreatedUnix int64
		Updated     time.Time `xorm:"-"`
		UpdatedUnix int64
	}
	if err = x.Sync2(new(ProtectedBranch)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	return nil
}
