// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func RecreateEmailHashTable(_ context.Context, x base.EngineMigration) error {
	// the rows are unreachable MD5 hashes and their UNIQUE email index would reject the SHA256 replacements
	if err := x.DropTables("email_hash"); err != nil {
		return err
	}

	type EmailHash struct {
		Hash     string `xorm:"pk varchar(64)"`
		Email    string `xorm:"UNIQUE(email_hashtype) NOT NULL"`
		HashType string `xorm:"UNIQUE(email_hashtype) NOT NULL varchar(16)"`
	}
	return x.Sync(new(EmailHash))
}
