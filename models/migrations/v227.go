// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"
	"strconv"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

type SystemSetting struct {
	ID           int64              `xorm:"pk autoincr"`
	SettingKey   string             `xorm:"varchar(255) unique"` // ensure key is always lowercase
	SettingValue string             `xorm:"text"`
	Version      int                `xorm:"version"` // prevent to override
	Created      timeutil.TimeStamp `xorm:"created"`
	Updated      timeutil.TimeStamp `xorm:"updated"`
}

func insertSettingsIfNotExist(x *xorm.Engine, sysSettings []*SystemSetting) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	for _, setting := range sysSettings {
		exist, err := sess.Table("system_setting").Where("setting_key=?", setting.SettingKey).Exist()
		if err != nil {
			return err
		}
		if !exist {
			if _, err := sess.Insert(setting); err != nil {
				return err
			}
		}
	}
	return sess.Commit()
}

func createSystemSettingsTable(x *xorm.Engine) error {
	if err := x.Sync2(new(SystemSetting)); err != nil {
		return fmt.Errorf("sync2: %w", err)
	}

	// migrate xx to database
	sysSettings := []*SystemSetting{
		{
			SettingKey:   "picture.disable_gravatar",
			SettingValue: strconv.FormatBool(setting.DisableGravatar),
		},
		{
			SettingKey:   "picture.enable_federated_avatar",
			SettingValue: strconv.FormatBool(setting.EnableFederatedAvatar),
		},
	}

	return insertSettingsIfNotExist(x, sysSettings)
}
