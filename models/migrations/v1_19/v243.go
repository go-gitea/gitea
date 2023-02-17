// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19 //nolint

import (
	"xorm.io/xorm"
)

// AddLFSSizeToRepositoryTable: add LFSSize column to Repository
func AddLFSSizeToRepositoryTable(x *xorm.Engine) error {
	type Repository struct {
		LFSSize int64 `xorm:"NOT NULL DEFAULT 0"`
	}

	return x.Sync2(new(Repository))
}
