// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/db"

	"xorm.io/xorm"
)

type VisibleType int

type teamWithVisibility struct {
	Visibility VisibleType `xorm:"NOT NULL DEFAULT 2"`
}

func (teamWithVisibility) TableName() string {
	return "team"
}

func AddVisibilityToTeam(x db.EngineMigration) error {
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(teamWithVisibility)); err != nil {
		return err
	}

	// Owner teams must remain listable to all org members; new orgs create
	// them as "limited", so make existing owner teams limited too.
	// Filter on authorize=4 (AccessModeOwner) so a user-created team that
	// happens to share the name "owners" is not accidentally affected.
	_, err := x.Exec("UPDATE `team` SET visibility = ? WHERE lower_name = ? AND authorize = ?", 1, "owners", 4)
	return err
}
