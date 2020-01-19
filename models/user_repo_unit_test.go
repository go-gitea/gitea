// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	// "fmt"
	"testing"

	// "github.com/stretchr/testify/assert"
)

func TestUserRepoUnitYaml(t *testing.T) {
	/*
	// **********************************************************************************************
	// *****					FIXME: GAP: move to contrib										*****
	// **********************************************************************************************

	assert.NoError(t, PrepareTestDatabase())

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

	if assert.NoError(t, x.Sync2(new(UserRepoUnit), new(UserRepoUnitWork), new(UserRepoUnitBatchNumber))) {
		assert.NoError(t, RebuildAllUserRepoUnits(x))

		units := make([]*UserRepoUnit, 0, 200)
		assert.NoError(t, x.OrderBy("user_id, repo_id").Find(&units))
		fmt.Printf("========================================\n")
		for _, u := range units {
			fmt.Printf("-\n")
			fmt.Printf("  user_id: %d\n", u.UserID)
			fmt.Printf("  repo_id: %d\n", u.RepoID)
			fmt.Printf("  type: %d\n", u.Type)
			fmt.Printf("  mode: %d\n", u.Mode)
			fmt.Printf("\n")
		}
		fmt.Printf("========================================\n")
		assert.Error(t, nil)
	}
	*/
}
