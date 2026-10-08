// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func AddEmailHashTable(_ context.Context, x base.EngineMigration) error {
	// EmailHash represents a pre-generated hash map
	type EmailHash struct {
		Hash  string `xorm:"pk varchar(32)"`
		Email string `xorm:"UNIQUE NOT NULL"`
	}
	return x.Sync(new(EmailHash))
}
