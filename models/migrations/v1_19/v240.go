// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func CreateRepoSettingsTable(x *xorm.Engine) error {
	type RepoSetting struct {
		ID           int64              `xorm:"pk autoincr"`
		RepoID       int64              `xorm:"index unique(key_repoid)"`              // to load all of someone's settings
		SettingKey   string             `xorm:"varchar(255) index unique(key_repoid)"` // ensure key is always lowercase
		SettingValue string             `xorm:"text"`
		Version      int                `xorm:"version"` // prevent to override
		Created      timeutil.TimeStamp `xorm:"created"`
		Updated      timeutil.TimeStamp `xorm:"updated"`
	}
	return x.Sync2(new(RepoSetting))
}
