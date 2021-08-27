// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

type UserSettings struct {
	ID     int64  `xorm:"pk autoincr"`
	UserID int64  `xorm:"index"`              // to load all of someone's settings
	Key    string `xorm:"varchar(255) index"` // ensure key is always lowercase
	Value  string `xorm:"text"`
}

// BeforeInsert will be invoked by XORM before inserting a record
func (setting *UserSettings) BeforeInsert() {
	setting.Key = strings.ToLower(setting.Key)
}

// GetUserSetting returns specific settings from user
func GetUserSetting(uid int64, keys []string) ([]*UserSettings, error) {
	settings := make([]*UserSettings, 0, 5)
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
func GetUserAllSettings(uid int64) ([]*UserSettings, error) {
	settings := make([]*UserSettings, 0, 5)
	if err := x.
		Where("uid=?", uid).
		Asc("id").
		Find(&settings); err != nil {
		return nil, err
	}
	return settings, nil
}

// AddUserSetting adds a specific setting for a user
func AddUserSetting(setting *UserSettings) error {
	return addUserSetting(x, setting)
}

func addUserSetting(e Engine, setting *UserSettings) error {
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

	return e.Where("key=?", strings.ToLower(key)).And("user_id = ?", uid).Get(&UserSettings{})
}

// DeleteUserSetting deletes a specific setting for a user
func DeleteUserSetting(setting *UserSettings) error {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if _, err = sess.Delete(setting); err != nil {
		return err
	}

	return sess.Commit()
}

// UpdateUserSetting updates a users' setting for a specific key
func UpdateUserSetting(setting *UserSettings) error {
	return updateUserSetting(x, setting)
}

func updateUserSetting(e Engine, setting *UserSettings) error {
	used, err := settingExists(e, setting.UserID, setting.Key)
	if err != nil {
		return err
	} else if !used {
		return ErrUserSettingNotExists{setting}
	}

	_, err := e.ID(u.ID).Cols("value").Update(setting)
	return err
}
