// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import "xorm.io/xorm"

func addEmailNotificationEnabledToUser(x *xorm.Engine) error {
	// User see models/user.go
	type User struct {
		EmailNotificationsPreference string `xorm:"VARCHAR(20) NOT NULL DEFAULT 'enabled'"`
	}

	return x.Sync2(new(User))
}
