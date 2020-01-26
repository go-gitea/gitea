// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import "xorm.io/xorm"

func addUnitsToRepoTeam(x *xorm.Engine) error {
	type Team struct {
		UnitTypes []int `xorm:"json"`
	}

	err := x.Sync(new(Team))
	if err != nil {
		return err
	}

	_, err = x.Update(&Team{
		UnitTypes: []int{1, 2, 3, 4, 5, 6, 7, 8, 9},
	})
	return err
}
