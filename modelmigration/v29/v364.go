// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddHashSHA256ToAttachment(_ context.Context, x base.EngineMigration) error {
	type Attachment struct {
		HashSHA256 string `xorm:"hash_sha256 char(64) NOT NULL DEFAULT ''"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(Attachment))
	return err
}
