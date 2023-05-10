// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import (
	"code.gitea.io/gitea/modules/log"

	"xorm.io/xorm"
)

func FixExternalTrackerAndExternalWikiAccessModeInOwnerAndAdminTeam(x *xorm.Engine) error {
	type UnitType int
	type AccessMode int

	type TeamUnit struct {
		ID         int64    `xorm:"pk autoincr"`
		Type       UnitType `xorm:"UNIQUE(s)"`
		AccessMode AccessMode
	}

	const (
		// AccessModeRead read access
		AccessModeRead = 1

		// Unit Type
		TypeExternalWiki    = 6
		TypeExternalTracker = 7
	)

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	count, err := sess.Table("team_unit").
		Where("type IN (?, ?) AND access_mode > ?", TypeExternalWiki, TypeExternalTracker, AccessModeRead).
		Update(&TeamUnit{
			AccessMode: AccessModeRead,
		})
	if err != nil {
		return err
	}
	log.Debug("Updated %d ExternalTracker and ExternalWiki access mode to belong to owner and admin", count)

	return sess.Commit()
}
