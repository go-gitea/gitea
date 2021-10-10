// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/builder"
)

// UserSetting is a key value store of user settings
type UserSetting struct {
	ID     int64  `xorm:"pk autoincr"`
	UserID int64  `xorm:"index unique(key_userid)"`              // to load all of someone's settings
	Key    string `xorm:"varchar(255) index unique(key_userid)"` // ensure key is always lowercase
	Value  string `xorm:"text"`
}

// BeforeInsert will be invoked by XORM before inserting a record
func (userSetting *UserSetting) BeforeInsert() {
	userSetting.Key = strings.ToLower(userSetting.Key)
}

// BeforeUpdate will be invoked by XORM before updating a record
func (userSetting *UserSetting) BeforeUpdate() {
	userSetting.Key = strings.ToLower(userSetting.Key)
}

// BeforeDelete will be invoked by XORM before updating a record
func (userSetting *UserSetting) BeforeDelete() {
	userSetting.Key = strings.ToLower(userSetting.Key)
}

func init() {
	db.RegisterModel(new(UserSetting))
}

// GetUserSetting returns specific settings from user
func GetUserSetting(uid int64, keys []string) ([]*UserSetting, error) {
	settings := make([]*UserSetting, 0, len(keys))
	if err := db.GetEngine(db.DefaultContext).
		Where("user_id=?", uid).
		And(builder.In("key", keys)).
		Asc("id").
		Find(&settings); err != nil {
		return nil, err
	}
	return settings, nil
}

// GetAllUserSettings returns all settings from user
func GetAllUserSettings(uid int64) ([]*UserSetting, error) {
	settings := make([]*UserSetting, 0, 5)
	if err := db.GetEngine(db.DefaultContext).
		Where("user_id=?", uid).
		Asc("id").
		Find(&settings); err != nil {
		return nil, err
	}
	return settings, nil
}

// DeleteUserSetting deletes a specific setting for a user
func DeleteUserSetting(userSetting *UserSetting) error {
	sess := db.NewSession(db.DefaultContext)
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if _, err := sess.Delete(userSetting); err != nil {
		return err
	}

	return sess.Commit()
}

// SetUserSetting updates a users' setting for a specific key
func SetUserSetting(userSetting *UserSetting) error {
	return upsertUserSettingValue(db.GetEngine(db.DefaultContext), userSetting.UserID, userSetting.Key, userSetting.Value)
}

func upsertUserSettingValue(e db.Engine, userID int64, key string, value string) (err error) {
	// Intentionally lowercase key here as XORM may not pick it up via Before* actions
	key = strings.ToLower(key)
	// An atomic UPSERT operation (INSERT/UPDATE) is the only operation
	// that ensures that the key is actually locked.
	switch {
	case setting.Database.UseSQLite3 || setting.Database.UsePostgreSQL:
		_, err = e.Exec("INSERT INTO `user_setting` (user_id, key, value) "+
			"VALUES (?,?,?) ON CONFLICT (user_id,key) DO UPDATE SET value = ?",
			userID, key, value, value)
	case setting.Database.UseMySQL:
		_, err = e.Exec("INSERT INTO `user_setting` (user_id, key, value) "+
			"VALUES (?,?,?) ON DUPLICATE KEY UPDATE value = ?",
			userID, key, value, value)
	case setting.Database.UseMSSQL:
		// https://weblogs.sqlteam.com/dang/2009/01/31/upsert-race-condition-with-merge/
		_, err = e.Exec("MERGE `user_setting` WITH (HOLDLOCK) as target "+
			"USING (SELECT ? AS user_id, ? AS key) AS src "+
			"ON src.user_id = target.user_id AND src.key = target.key "+
			"WHEN MATCHED THEN UPDATE SET target.value = ? "+
			"WHEN NOT MATCHED THEN INSERT (user_id, key, value) "+
			"VALUES (src.user_id, src.key, ?);",
			userID, key, value, value)
	default:
		return fmt.Errorf("database type not supported")
	}
	return
}
