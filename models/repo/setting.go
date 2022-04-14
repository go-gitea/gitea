// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"

	"xorm.io/builder"
)

// Setting is a key value store of repo settings
type Setting struct {
	ID           int64  `xorm:"pk autoincr"`
	RepoID       int64  `xorm:"index unique(key_repoid)"`              // to load all of someone's settings
	SettingKey   string `xorm:"varchar(255) index unique(key_repoid)"` // ensure key is always lowercase
	SettingValue string `xorm:"text"`
}

// TableName sets the table name for the settings struct
func (s *Setting) TableName() string {
	return "repo_setting"
}

func init() {
	db.RegisterModel(new(Setting))
}

// GetRepoSettings returns specific settings from repo
func GetRepoSettings(rid int64, keys []string) (map[string]*Setting, error) {
	settings := make([]*Setting, 0, len(keys))
	if err := db.GetEngine(db.DefaultContext).
		Where("repo_id=?", rid).
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

// GetRepoAllSettings returns all settings from repo
func GetRepoAllSettings(rid int64) (map[string]*Setting, error) {
	settings := make([]*Setting, 0, 5)
	if err := db.GetEngine(db.DefaultContext).
		Where("repo_id=?", rid).
		Find(&settings); err != nil {
		return nil, err
	}
	settingsMap := make(map[string]*Setting)
	for _, s := range settings {
		settingsMap[s.SettingKey] = s
	}
	return settingsMap, nil
}

func validateRepoSettingKey(key string) error {
	if len(key) == 0 {
		return fmt.Errorf("setting key must be set")
	}
	if strings.ToLower(key) != key {
		return fmt.Errorf("setting key should be lowercase")
	}
	return nil
}

// GetRepoSetting gets a specific setting for a repo
func GetRepoSetting(repoID int64, key string, def ...string) (string, error) {
	if err := validateRepoSettingKey(key); err != nil {
		return "", err
	}
	setting := &Setting{RepoID: repoID, SettingKey: key}
	has, err := db.GetEngine(db.DefaultContext).Get(setting)
	if err != nil {
		return "", err
	}
	if !has {
		if len(def) == 1 {
			return def[0], nil
		}
		return "", nil
	}
	return setting.SettingValue, nil
}

// DeleteRepoSetting deletes a specific setting for a repo
func DeleteRepoSetting(repoID int64, key string) error {
	if err := validateRepoSettingKey(key); err != nil {
		return err
	}
	_, err := db.GetEngine(db.DefaultContext).Delete(&Setting{RepoID: repoID, SettingKey: key})
	return err
}

// SetRepoSetting updates a repos' setting for a specific key
func SetRepoSetting(repoID int64, key, value string) error {
	if err := validateRepoSettingKey(key); err != nil {
		return err
	}
	return upsertRepoSettingValue(repoID, key, value)
}

func upsertRepoSettingValue(repoID int64, key, value string) error {
	return db.WithTx(func(ctx context.Context) error {
		e := db.GetEngine(ctx)

		// here we use a general method to do a safe upsert for different databases (and most transaction levels)
		// 1. try to UPDATE the record and acquire the transaction write lock
		//    if UPDATE returns non-zero rows are changed, OK, the setting is saved correctly
		//    if UPDATE returns "0 rows changed", two possibilities: (a) record doesn't exist  (b) value is not changed
		// 2. do a SELECT to check if the row exists or not (we already have the transaction lock)
		// 3. if the row doesn't exist, do an INSERT (we are still protected by the transaction lock, so it's safe)
		//
		// to optimize the SELECT in step 2, we can use an extra column like `revision=revision+1`
		//    to make sure the UPDATE always returns a non-zero value for existing (unchanged) records.

		res, err := e.Exec("UPDATE repo_setting SET setting_value=? WHERE setting_key=? AND repo_id=?", value, key, repoID)
		if err != nil {
			return err
		}
		rows, _ := res.RowsAffected()
		if rows > 0 {
			// the existing row is updated, so we can return
			return nil
		}

		// in case the value isn't changed, update would return 0 rows changed, so we need this check
		has, err := e.Exist(&Setting{RepoID: repoID, SettingKey: key})
		if err != nil {
			return err
		}
		if has {
			return nil
		}

		// if no existing row, insert a new row
		_, err = e.Insert(&Setting{RepoID: repoID, SettingKey: key, SettingValue: value})
		return err
	})
}
