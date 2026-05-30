// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"code.gitea.io/gitea/models/db"

	"xorm.io/xorm"
)

type teamWithPrivacy struct {
	TeamPrivacy string `xorm:"VARCHAR(16) NOT NULL DEFAULT 'private'"`
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

	// Pre-release deployments of this PR persisted GitHub-style "secret"/
	// "closed" values; rewrite them to the new vocabulary so the migration
	// is idempotent across rebases.
	if _, err := x.Exec("UPDATE `team` SET team_privacy = ? WHERE team_privacy = ?", "private", "secret"); err != nil {
		return err
	}
	if _, err := x.Exec("UPDATE `team` SET team_privacy = ? WHERE team_privacy = ?", "limited", "closed"); err != nil {
		return err
	}

	// Owner teams must remain listable to all org members; new orgs create
	// them as "limited", so make existing owner teams limited too.
	_, err := x.Exec("UPDATE `team` SET team_privacy = ? WHERE lower_name = ?", "limited", "owners")
	return err
}
