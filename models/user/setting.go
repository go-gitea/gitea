// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"

	"xorm.io/builder"
)

// Setting is a key value store of user settings
type Setting struct {
	ID           int64  `xorm:"pk autoincr"`
	UserID       int64  `xorm:"index unique(key_userid)"`              // to load all of someone's settings
	SettingKey   string `xorm:"varchar(255) index unique(key_userid)"` // ensure key is always lowercase
	SettingValue string `xorm:"text"`
}

// TableName sets the table name for the settings struct
func (s *Setting) TableName() string {
	return "user_setting"
}

func init() {
	db.RegisterModel(new(Setting))
}

// GetSetting returns specific settings from user
// func GetSetting(uid int64, keys []string) ([]*Setting, error) {
func GetSetting(uid int64, keys []string) (map[string]*Setting, error) {
	settings := make([]*Setting, 0, len(keys))
	if err := db.GetEngine(db.DefaultContext).
		Where("user_id=?", uid).
		And(builder.In("setting_key", keys)).
		Find(&settings); err != nil {
		return nil, err
	}
	settingsMap := make(map[string]*Setting)
	for _, s := range settings {
		settingsMap[s.SettingKey] = s
	}
	return settingsMap, nil
}

// GetUserAllSettings returns all settings from user
func GetUserAllSettings(uid int64) (map[string]*Setting, error) {
	settings := make([]*Setting, 0, 5)
	if err := db.GetEngine(db.DefaultContext).
		Where("user_id=?", uid).
		Asc("id").
		Find(&settings); err != nil {
		return nil, err
	}
	settingsMap := make(map[string]*Setting)
	for _, s := range settings {
		settingsMap[s.SettingKey] = s
	}
	return settingsMap, nil
}

// DeleteSetting deletes a specific setting for a user
func DeleteSetting(setting *Setting) error {
	sess := db.GetEngine(db.DefaultContext)

	_, err := sess.Delete(setting)
	return err
}

// SetSetting updates a users' setting for a specific key
func SetSetting(setting *Setting) error {
	if strings.ToLower(setting.SettingKey) != setting.SettingKey {
		return fmt.Errorf("setting key should be lowercase")
	}
	return upsertSettingValue(db.GetEngine(db.DefaultContext), setting.UserID, setting.SettingKey, setting.SettingValue)
}

func upsertSettingValue(e db.Engine, userID int64, key string, value string) error {
	return db.WithTx(func(ctx context.Context) error {
		sess := db.GetEngine(db.DefaultContext)
		res, err := sess.Exec("UPDATE user_setting SET setting_value=? WHERE setting_key=?", value, key)
		if err != nil {
			return err
		}
		rows, _ := res.RowsAffected()
		if rows != 0 {
			// the existing row is updated, so we can return
			return nil
		}
		// if no existing row, insert a new row
		_, err = sess.Insert(&Setting{SettingKey: key, SettingValue: value})
		return err
	})
}
