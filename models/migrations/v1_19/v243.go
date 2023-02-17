// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19 //nolint

import (
	"xorm.io/xorm"
)

type SizeDetails struct {
	GitSize int64
	LFSSize int64
	// TODO: size of more parts.
}

// AddSizeDetailsToRepositoryTable: add LFSSize column to Repository
func AddSizeDetailsToRepositoryTable(x *xorm.Engine) error {
	type Repository struct {
		SizeDetails SizeDetails `xorm:"TEXT JSON"`
	}

	return x.Sync2(new(Repository))
}
