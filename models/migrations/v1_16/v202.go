// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16

import (
	"fmt"

	"gitea.dev/models/db"
)

func CreateUserSettingsTable(x db.EngineMigration) error {
	type UserSetting struct {
		ID           int64  `xorm:"pk autoincr"`
		UserID       int64  `xorm:"index unique(key_userid)"`              // to load all of someone's settings
		SettingKey   string `xorm:"varchar(255) index unique(key_userid)"` // ensure key is always lowercase
		SettingValue string `xorm:"text"`
	}
	if err := x.Sync(new(UserSetting)); err != nil {
		return fmt.Errorf("sync2: %w", err)
	}
	return nil
}
