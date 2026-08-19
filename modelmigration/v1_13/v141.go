// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_13

import (
	"context"
	"fmt"

	"gitea.dev/modelmigration/base"
)

func AddKeepActivityPrivateUserColumn(_ context.Context, x base.EngineMigration) error {
	type User struct {
		KeepActivityPrivate bool `xorm:"NOT NULL DEFAULT false"`
	}

	if err := x.Sync(new(User)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	return nil
}
