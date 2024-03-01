// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_10 //nolint

import "xorm.io/xorm"

func AddEmailNotificationEnabledToUser(x *xorm.Engine) error {
	// User see models/user.go
	type User struct {
		EmailNotificationsPreference string `xorm:"VARCHAR(20) NOT NULL DEFAULT 'enabled'"`
	}

	return x.Sync(new(User))
}
