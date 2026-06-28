// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_28

import "gitea.dev/models/db"

func AddChecksumsToAttachment(x db.EngineMigration) error {
	type Attachment struct {
		ID         int64  `xorm:"pk autoincr"`
		HashSHA256 string `xorm:"hash_sha256 VARCHAR(64) NOT NULL DEFAULT ''"`
	}
	return x.Sync(new(Attachment))
}
