// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/builder"
)

// Setting is a key value store of user settings
type Setting struct {
	ID           int64  `xorm:"pk autoincr"`
	UserID       int64  `xorm:"index unique(key_userid)"`              // to load all of someone's settings
	SettingKey   string `xorm:"varchar(255) index unique(key_userid)"` // ensure key is always lowercase
	SettingValue string `xorm:"text"`
}

func (Setting) TableName() string {
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
func DeleteSetting(Setting *Setting) error {
	sess := db.GetEngine(db.DefaultContext)

	_, err := sess.Delete(Setting)
	return err
}

// SetSetting updates a users' setting for a specific key
func SetSetting(Setting *Setting) error {
	if strings.ToLower(Setting.SettingKey) != Setting.SettingKey {
		return fmt.Errorf("setting key should be lowercase")
	}
	return upsertSettingValue(db.GetEngine(db.DefaultContext), Setting.UserID, Setting.SettingKey, Setting.SettingValue)
}

func upsertSettingValue(e db.Engine, userID int64, key string, value string) (err error) {
	// Intentionally lowercase key here as XORM may not pick it up via Before* actions
	key = strings.ToLower(key)
	// An atomic UPSERT operation (INSERT/UPDATE) is the only operation
	// that ensures that the key is actually locked.
	switch {
	case setting.Database.UseSQLite3 || setting.Database.UsePostgreSQL:
		_, err = e.Exec("INSERT INTO `user_setting` (user_id, setting_key, setting_value) "+
			"VALUES (?,?,?) ON CONFLICT (user_id,setting_key) DO UPDATE SET setting_value = ?",
			userID, key, value, value)
	case setting.Database.UseMySQL:
		_, err = e.Exec("INSERT INTO `user_setting` (user_id, setting_key, setting_value) "+
			"VALUES (?,?,?) ON DUPLICATE KEY UPDATE setting_value = ?",
			userID, key, value, value)
	case setting.Database.UseMSSQL:
		// https://weblogs.sqlteam.com/dang/2009/01/31/upsert-race-condition-with-merge/
		_, err = e.Exec("MERGE `user_setting` WITH (HOLDLOCK) as target "+
			"USING (SELECT ? AS user_id, ? AS setting_key) AS src "+
			"ON src.user_id = target.user_id AND src.setting_key = target.setting_key "+
			"WHEN MATCHED THEN UPDATE SET target.setting_value = ? "+
			"WHEN NOT MATCHED THEN INSERT (user_id, setting_key, setting_value) "+
			"VALUES (src.user_id, src.setting_key, ?);",
			userID, key, value, value)
	default:
		return fmt.Errorf("database type not supported")
	}
	return
}
