// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_10

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func AddEmailNotificationEnabledToUser(_ context.Context, x base.EngineMigration) error {
	// User see models/user.go
	type User struct {
		EmailNotificationsPreference string `xorm:"VARCHAR(20) NOT NULL DEFAULT 'enabled'"`
	}

	return x.Sync(new(User))
}
