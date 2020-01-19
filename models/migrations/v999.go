// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
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

	type LockedResource struct {
		Type	string      `xorm:"pk"`
		Key		int64		`xorm:"pk"`
		Counter	int64		`xorm:"NOT NULL DEFAULT 0"`
	}
	
	// FIXME: GAP: This needs its own migration
	type Repository struct {
		IsArchived          bool         `xorm:"INDEX"`
		Topics              []string     `xorm:"TEXT JSON"`
	}
	type VisibleType int
	type User struct {
		PasswdHashAlgo      string       `xorm:"NOT NULL DEFAULT 'pbkdf2'"`
		Visibility          VisibleType  `xorm:"NOT NULL DEFAULT 0"`
	}
	if err := x.Sync2(new(Repository), new(User)); err != nil {
		return err
	}

	// Rebuilding the permissions cache must be performed after all migrations were done
	// because it is implemented in the models package using the latest form of all tables.
	// The tables might suffer changes in migration steps after this one, so it's not safe
	// to call models.RebuildAllUserRepoUnits() just yet.
	RebuildPermissionsRequired = true

	return x.Sync2(
		new(UserRepoUnit),
		new(UserRepoUnitWork),
		new(UserRepoUnitBatchNumber),
		new(LockedResource))
}
