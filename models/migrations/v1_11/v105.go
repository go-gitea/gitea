// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_11

import "gitea.dev/models/db"

func AddTeamIncludesAllRepositories(x db.EngineMigration) error {
	type Team struct {
		ID                      int64 `xorm:"pk autoincr"`
		IncludesAllRepositories bool  `xorm:"NOT NULL DEFAULT false"`
	}

	if err := x.Sync(new(Team)); err != nil {
		return err
	}

	_, err := x.Exec("UPDATE `team` SET `includes_all_repositories` = ? WHERE `name`=?",
		true, "Owners")
	return err
}
