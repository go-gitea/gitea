// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint

import (
	"xorm.io/xorm"
)

// AddGitSizeAndLFSSizeToRepositoryTable: add GitSize and LFSSize columns to Repository
func AddGitSizeAndLFSSizeToRepositoryTable(x *xorm.Engine) error {
	type Repository struct {
		GitSize int64 `xorm:"NOT NULL DEFAULT 0"`
		LFSSize int64 `xorm:"NOT NULL DEFAULT 0"`
	}

	return x.Sync2(new(Repository))
}
