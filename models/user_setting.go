// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"strings"

	"xorm.io/builder"
)

// UserSetting is a key value store of user settings
type UserSetting struct {
	ID     int64  `xorm:"pk autoincr"`
	UserID int64  `xorm:"index"`              // to load all of someone's settings
	Key    string `xorm:"varchar(255) index"` // ensure key is always lowercase
	Value  string `xorm:"text"`
}

// BeforeInsert will be invoked by XORM before inserting a record
func (setting *UserSetting) BeforeInsert() {
	setting.Key = strings.ToLower(setting.Key)
}

// GetUserSetting returns specific settings from user
func GetUserSetting(uid int64, keys []string) ([]*UserSetting, error) {
	settings := make([]*UserSetting, 0, 5)
	if err := x.
		Where("uid=?", uid).
		And(builder.In("key", keys)).
		Asc("id").
		Find(&settings); err != nil {
		return nil, err
	}
	return settings, nil
}

// GetUserAllSettings returns all settings from user
func GetUserAllSettings(uid int64) ([]*UserSetting, error) {
	settings := make([]*UserSetting, 0, 5)
	if err := x.
		Where("uid=?", uid).
		Asc("id").
		Find(&settings); err != nil {
		return nil, err
	}
	return settings, nil
}

// AddUserSetting adds a specific setting for a user
func AddUserSetting(setting *UserSetting) error {
	return addUserSetting(x, setting)
}

func addUserSetting(e Engine, setting *UserSetting) error {
	used, err := settingExists(e, setting.UserID, setting.Key)
	if err != nil {
		return err
	} else if used {
		return ErrUserSettingExists{setting}
	}
	_, err = e.Insert(setting)
	return err
}

func settingExists(e Engine, uid int64, key string) (bool, error) {
	if len(key) == 0 {
		return true, nil
	}

	return e.Where("key=?", strings.ToLower(key)).And("user_id = ?", uid).Get(&UserSetting{})
}

// DeleteUserSetting deletes a specific setting for a user
func DeleteUserSetting(setting *UserSetting) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if _, err := sess.Delete(setting); err != nil {
		return err
	}

	return sess.Commit()
}

// UpdateUserSettingValue updates a users' setting for a specific key
func UpdateUserSettingValue(setting *UserSetting) error {
	return updateUserSettingValue(x, setting)
}

func updateUserSettingValue(e Engine, setting *UserSetting) error {
	used, err := settingExists(e, setting.UserID, setting.Key)
	if err != nil {
		return err
	} else if !used {
		return ErrUserSettingNotExists{setting}
	}

	_, err = e.ID(setting.ID).Cols("value").Update(setting)
	return err
}
