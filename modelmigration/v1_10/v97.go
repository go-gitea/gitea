// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_10

import "gitea.dev/modelmigration/base"

func AddRepoAdminChangeTeamAccessColumnForUser(x base.EngineMigration) error {
	type User struct {
		RepoAdminChangeTeamAccess bool `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(new(User))
}
