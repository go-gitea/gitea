// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import (
	"code.gitea.io/gitea/modules/log"

	"xorm.io/xorm"
)

func FixIncorrectAdminTeamUnitAccessMode(x *xorm.Engine) error {
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
		// AccessModeAdmin admin access
		AccessModeAdmin = 3
	)

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	count, err := sess.Table("team_unit").
		Where("team_id IN (SELECT id FROM team WHERE authorize = ?)", AccessModeAdmin).
		Update(&TeamUnit{
			AccessMode: AccessModeAdmin,
		})
	if err != nil {
		return err
	}
	log.Debug("Updated %d admin team unit access mode to belong to admin instead of none", count)

	return sess.Commit()
}
