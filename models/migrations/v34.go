// Copyright 2017 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

// Team see models/team.go
type Team struct {
	UnitTypes []int `xorm:"json"`
}

const ownerAccessMode = 4

var allUnitTypes = []int{1, 2, 3, 4, 5, 6, 7, 8, 9}

func giveAllUnitsToOwnerTeams(x *xorm.Engine) error {
	_, err := x.Cols("unit_types").
		Where("authorize = ?", ownerAccessMode).
		Update(&Team{UnitTypes: allUnitTypes})
	return err
}
