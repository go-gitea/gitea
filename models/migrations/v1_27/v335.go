// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"code.gitea.io/gitea/models/db"

	"xorm.io/xorm"
)

type teamWithPrivacy struct {
	TeamPrivacy string `xorm:"VARCHAR(16) NOT NULL DEFAULT 'secret'"`
}

func (teamWithPrivacy) TableName() string {
	return "team"
}

func AddPrivacyToTeam(x db.EngineMigration) error {
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
	}, new(teamWithPrivacy)); err != nil {
		return err
	}

	// Owner teams must remain listable to all org members; new orgs create
	// them as "closed", so make existing owner teams closed too.
	_, err := x.Exec("UPDATE `team` SET team_privacy = ? WHERE lower_name = ?", "closed", "owners")
	return err
}
