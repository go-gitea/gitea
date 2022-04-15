// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func createRepoSettingsTable(x *xorm.Engine) error {
	type RepoSetting struct {
		ID           int64  `xorm:"pk autoincr"`
		RepoID       int64  `xorm:"index unique(key_repoid)"`              // to load all of someone's settings
		SettingKey   string `xorm:"varchar(255) index unique(key_repoid)"` // ensure key is always lowercase
		SettingValue string `xorm:"text"`
	}
	return x.Sync2(new(RepoSetting))
}
