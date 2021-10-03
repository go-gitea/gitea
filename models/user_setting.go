// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"strings"

	"code.gitea.io/gitea/models/db"

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
func (setting *UserSetting) BeforeInsert() {
	setting.Key = strings.ToLower(setting.Key)
}

// BeforeUpdate will be invoked by XORM before updating a record
func (setting *UserSetting) BeforeUpdate() {
	setting.Key = strings.ToLower(setting.Key)
}

func init() {
	db.RegisterModel(new(UserSetting))
}

// GetUserSetting returns specific settings from user
func GetUserSetting(uid int64, keys []string) ([]*UserSetting, error) {
	settings := make([]*UserSetting, 0, 5)
	if err := db.GetEngine(db.DefaultContext).
		Where("user_id=?", uid).
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
	if err := db.GetEngine(db.DefaultContext).
		Where("user_id=?", uid).
		Asc("id").
		Find(&settings); err != nil {
		return nil, err
	}
	return settings, nil
}

func addUserSetting(e db.Engine, setting *UserSetting) error {
	used, err := settingExists(e, setting.UserID, setting.Key)
	if err != nil {
		return err
	} else if used {
		return ErrUserSettingExists{setting}
	}
	_, err = e.Insert(setting)
	return err
}

func settingExists(e db.Engine, uid int64, key string) (bool, error) {
	if len(key) == 0 {
		return true, nil
	}

	return e.Table(&UserSetting{}).Exist(&UserSetting{UserID: uid, Key: strings.ToLower(key)})
}

// DeleteUserSetting deletes a specific setting for a user
func DeleteUserSetting(setting *UserSetting) error {
	sess := db.NewSession(db.DefaultContext)
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if _, err := sess.Delete(setting); err != nil {
		return err
	}

	return sess.Commit()
}

// SetUserSetting updates a users' setting for a specific key
func SetUserSetting(setting *UserSetting) error {
	err := addUserSetting(db.GetEngine(db.DefaultContext), setting)
	if err != nil && IsErrUserSettingExists(err) {
		return updateUserSettingValue(db.GetEngine(db.DefaultContext), setting)
	}
	return err
}

func updateUserSettingValue(e db.Engine, setting *UserSetting) error {
	used, err := settingExists(e, setting.UserID, setting.Key)
	if err != nil {
		return err
	} else if !used {
		return ErrUserSettingNotExists{setting}
	}

	_, err = e.ID(setting.ID).Cols("value").Update(setting)
	return err
}
