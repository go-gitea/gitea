// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_10 //nolint

import "xorm.io/xorm"

func AddRepoAdminChangeTeamAccessColumnForUser(x *xorm.Engine) error {
	type User struct {
		RepoAdminChangeTeamAccess bool `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(new(User))
}
