// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20

import "gitea.dev/modelmigration/base"

func AddNewColumnForProject(x base.EngineMigration) error {
	type Project struct {
		OwnerID int64 `xorm:"INDEX"`
	}

	return x.Sync(new(Project))
}
