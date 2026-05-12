// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import "xorm.io/xorm"

type teamWithVisibility struct {
	Visibility string `xorm:"VARCHAR(16) NOT NULL DEFAULT 'secret'"`
}

func (teamWithVisibility) TableName() string {
	return "team"
}

func AddVisibilityToTeam(x *xorm.Engine) error {
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
	}, new(teamWithVisibility)); err != nil {
		return err
	}

	// Owner teams must remain listable to all org members; new orgs create
	// them as visible, so make existing owner teams visible too.
	_, err := x.Exec("UPDATE `team` SET visibility = ? WHERE lower_name = ?", "visible", "owners")
	return err
}
