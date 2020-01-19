// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/models"

	"xorm.io/xorm"
)

// IMPORTANT: THIS MIGRATION DOES NOT HAVE IT'S FINAL NUMBER OR FORM.
// IT'S NOT INTENDED TO GO LIKE THIS IN THE FINAL VERSION OF THE PR.

func addUserRepoUnit(x *xorm.Engine) error {

	// AccessMode is Unit's Type
	type AccessMode int
	type UnitType int

	type UserRepoUnit struct {
		UserID int64      `xorm:"pk"`
		RepoID int64      `xorm:"pk INDEX"`
		Type   UnitType   `xorm:"pk"`
		Mode   AccessMode `xorm:"NOT NULL"`
	}

	type UserRepoUnitWork struct {
		BatchID int64      `xorm:"NOT NULL INDEX"`
		UserID  int64      `xorm:"NOT NULL"`
		RepoID  int64      `xorm:"NOT NULL"`
		Type    UnitType   `xorm:"NOT NULL"`
		Mode    AccessMode `xorm:"NOT NULL"`
	}

	type UserRepoUnitBatchNumber struct {
		ID int64 `xorm:"pk autoincr"`
	}

	if err := x.Sync2(new(UserRepoUnit), new(UserRepoUnitWork), new(UserRepoUnitBatchNumber)); err != nil {
		return err
	}

	// FIXME: GAP: Rebuilding permissions cache must be added as a final step for
	// all migrations because it uses the latest form of all tables, which may
	// be changing in migrations after this one.

	return models.RebuildAllUserRepoUnits(x)
}
