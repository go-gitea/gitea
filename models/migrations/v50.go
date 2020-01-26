// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func migrateProtectedBranchStruct(x *xorm.Engine) error {
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

	var pbs []ProtectedBranch
	err := x.Find(&pbs)
	if err != nil {
		return err
	}

	for _, pb := range pbs {
		if pb.CanPush {
			if _, err = x.ID(pb.ID).Delete(new(ProtectedBranch)); err != nil {
				return err
			}
		}
	}

	switch {
	case setting.Database.UseSQLite3:
		log.Warn("Unable to drop columns in SQLite")
	case setting.Database.UseMySQL, setting.Database.UsePostgreSQL, setting.Database.UseMSSQL:
		if _, err := x.Exec("ALTER TABLE protected_branch DROP COLUMN can_push"); err != nil {
			// Ignoring this error in case we run this migration second time (after migration reordering)
			log.Warn("DROP COLUMN can_push (skipping): %v", err)
		}
	default:
		log.Fatal("Unrecognized DB")
	}

	return nil
}
