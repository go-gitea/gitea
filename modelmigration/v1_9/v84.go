// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_9

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func AddGPGKeyImport(_ context.Context, x base.EngineMigration) error {
	type GPGKeyImport struct {
		KeyID   string `xorm:"pk CHAR(16) NOT NULL"`
		Content string `xorm:"TEXT NOT NULL"`
	}

	return x.Sync(new(GPGKeyImport))
}
