// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_24

import (
	"code.gitea.io/gitea/models/db"

	"xorm.io/xorm"
)

func AddDescriptionForSecretsAndVariables(x db.EngineMigration) error {
	type Secret struct {
		Description string `xorm:"TEXT"`
	}

	type ActionVariable struct {
		Description string `xorm:"TEXT"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains: true,
		IgnoreIndices:    true,
	}, new(Secret), new(ActionVariable))
	return err
}
