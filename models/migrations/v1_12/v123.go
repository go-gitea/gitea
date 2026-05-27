// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12

import "gitea.dev/models/db"

func AddReactionOriginals(x db.EngineMigration) error {
	type Reaction struct {
		OriginalAuthorID int64 `xorm:"INDEX NOT NULL DEFAULT(0)"`
		OriginalAuthor   string
	}

	return x.Sync(new(Reaction))
}
