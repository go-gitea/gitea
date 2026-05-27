// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_10

import "gitea.dev/models/db"

func AddOriginalAuthorOnMigratedReleases(x db.EngineMigration) error {
	type Release struct {
		ID               int64
		OriginalAuthor   string
		OriginalAuthorID int64 `xorm:"index"`
	}

	return x.Sync(new(Release))
}
