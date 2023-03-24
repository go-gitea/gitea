// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import (
	"code.gitea.io/gitea/modules/log"

	"xorm.io/xorm"
)

func FixIncorrectOwnerTeamUnitAccessMode(x *xorm.Engine) error {
	type UnitType int
	type AccessMode int

	type TeamUnit struct {
		ID         int64    `xorm:"pk autoincr"`
		OrgID      int64    `xorm:"INDEX"`
		TeamID     int64    `xorm:"UNIQUE(s)"`
		Type       UnitType `xorm:"UNIQUE(s)"`
		AccessMode AccessMode
	}

	const (
		// AccessModeOwner owner access
		AccessModeOwner = 4
	)

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	count, err := sess.Table("team_unit").
		Where("team_id IN (SELECT id FROM team WHERE authorize = ?)", AccessModeOwner).
		Update(&TeamUnit{
			AccessMode: AccessModeOwner,
		})
	if err != nil {
		return err
	}
	log.Debug("Updated %d owner team unit access mode to belong to owner instead of none", count)

	return sess.Commit()
}
