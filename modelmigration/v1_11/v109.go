// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_11

import "gitea.dev/modelmigration/base"

func AddCanCreateOrgRepoColumnForTeam(x base.EngineMigration) error {
	type Team struct {
		CanCreateOrgRepo bool `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(new(Team))
}
