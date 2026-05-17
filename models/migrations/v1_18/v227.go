// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_18

import (
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/models/db"

)

type SystemSetting struct {
	ID           int64              `xorm:"pk autoincr"`
	SettingKey   string             `xorm:"varchar(255) unique"` // ensure key is always lowercase
	SettingValue string             `xorm:"text"`
	Version      int                `xorm:"version"` // prevent to override
	Created      timeutil.TimeStamp `xorm:"created"`
	Updated      timeutil.TimeStamp `xorm:"updated"`
}

func CreateSystemSettingsTable(x db.EngineMigration) error {
	return x.Sync(new(SystemSetting))
}
