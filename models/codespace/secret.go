// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import "gitea.dev/models/db"

// UserSecret stores an encrypted Codespace environment variable owned by one user.
type UserSecret struct {
	ID            int64
	UserID        int64  `xorm:"NOT NULL unique(user_name)"`
	Name          string `xorm:"VARCHAR(255) NOT NULL unique(user_name)"`
	DataEncrypted string `xorm:"LONGTEXT NOT NULL"`
	CreatedUnix   int64  `xorm:"NOT NULL DEFAULT 0"`
	UpdatedUnix   int64  `xorm:"NOT NULL DEFAULT 0"`
}

func (*UserSecret) TableName() string {
	return "codespace_user_secret"
}

func init() {
	db.RegisterModel(new(UserSecret))
}
