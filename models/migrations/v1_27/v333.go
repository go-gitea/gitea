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
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
	}, new(teamWithVisibility))
	return err
}
