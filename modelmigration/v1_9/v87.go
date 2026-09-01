// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_9

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func AddAvatarFieldToRepository(_ context.Context, x base.EngineMigration) error {
	type Repository struct {
		// ID(10-20)-md5(32) - must fit into 64 symbols
		Avatar string `xorm:"VARCHAR(64)"`
	}

	return x.Sync(new(Repository))
}
