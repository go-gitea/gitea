// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_8

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func AddUserDefaultTheme(_ context.Context, x base.EngineMigration) error {
	type User struct {
		Theme string `xorm:"VARCHAR(30) NOT NULL DEFAULT ''"`
	}

	return x.Sync(new(User))
}
