// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func createSystemSettingsTable(x *xorm.Engine) error {
	type SystemSetting struct {
		ID           int64  `xorm:"pk autoincr"`
		SettingKey   string `xorm:"varchar(255) unique"` // ensure key is always lowercase
		SettingValue string `xorm:"text"`
	}
	if err := x.Sync2(new(SystemSetting)); err != nil {
		return fmt.Errorf("sync2: %v", err)
	}
	return nil
}
