// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/cache"
	setting_module "code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// genSettingCacheKey returns the cache key for some configuration
func genSettingCacheKey(repoID int64, key string) string {
	return fmt.Sprintf("repo.setting.%d.%s", repoID, key)
}

// Setting is a key value store of repo settings
type Setting struct {
	ID           int64              `xorm:"pk autoincr"`
	RepoID       int64              `xorm:"index unique(key_repoid)"`              // to load all of someone's settings
	SettingKey   string             `xorm:"varchar(255) index unique(key_repoid)"` // ensure key is always lowercase
	SettingValue string             `xorm:"text"`
	Version      int                `xorm:"version"` // prevent to override
	Created      timeutil.TimeStamp `xorm:"created"`
	Updated      timeutil.TimeStamp `xorm:"updated"`
}

// TableName sets the table name for the settings struct
func (s *Setting) TableName() string {
	return "repo_setting"
}

func (s *Setting) GetValueBool() bool {
	if s == nil {
		return false
	}

	b, _ := strconv.ParseBool(s.SettingValue)
	return b
}

func init() {
	db.RegisterModel(new(Setting))
}

// GetSettingNoCache returns specific setting without using the cache
func GetSettingNoCache(repoID int64, key string) (*Setting, error) {
	v, err := GetSettings(repoID, []string{key})
	if err != nil {
		return nil, err
	}
	if len(v) == 0 {
		return nil, fmt.Errorf("repo[%d] setting[%s]: %w", repoID, key, util.ErrNotExist)
	}
	return v[strings.ToLower(key)], nil
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

// GetSetting returns the setting value via the key
func GetSetting(repoID int64, key string) (string, error) {
	if err := validateRepoSettingKey(key); err != nil {
		return "", err
	}
	return cache.GetString(genSettingCacheKey(repoID, key), func() (string, error) {
		res, err := GetSettingNoCache(repoID, key)
		if err != nil {
			return "", err
		}
		return res.SettingValue, nil
	})
}

// GetSettingBool return bool value of setting,
// none existing keys and errors are ignored and result in false
func GetSettingBool(repoID int64, key string) bool {
	s, _ := GetSetting(repoID, key)
	v, _ := strconv.ParseBool(s)
	return v
}

// GetSettings returns specific settings
func GetSettings(repoID int64, keys []string) (map[string]*Setting, error) {
	for i := 0; i < len(keys); i++ {
		keys[i] = strings.ToLower(keys[i])
	}
	settings := make([]*Setting, 0, len(keys))
	if err := db.GetEngine(db.DefaultContext).
		Where("repo_id=?", repoID).
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

type AllSettings map[string]*Setting

func (settings AllSettings) Get(key string) Setting {
	if v, ok := settings[strings.ToLower(key)]; ok {
		return *v
	}
	return Setting{}
}

func (settings AllSettings) GetBool(key string) bool {
	b, _ := strconv.ParseBool(settings.Get(key).SettingValue)
	return b
}

func (settings AllSettings) GetVersion(key string) int {
	return settings.Get(key).Version
}

// GetAllSettings returns all settings from repo
func GetAllSettings(repoID int64) (AllSettings, error) {
	settings := make([]*Setting, 0, 5)
	if err := db.GetEngine(db.DefaultContext).
		Where("repo_id=?", repoID).
		Find(&settings); err != nil {
		return nil, err
	}
	settingsMap := make(map[string]*Setting)
	for _, s := range settings {
		settingsMap[s.SettingKey] = s
	}
	return settingsMap, nil
}

// DeleteSetting deletes a specific setting for a repo
func DeleteSetting(repoID int64, key string) error {
	if err := validateRepoSettingKey(key); err != nil {
		return err
	}
	cache.Remove(genSettingCacheKey(repoID, key))
	_, err := db.GetEngine(db.DefaultContext).Delete(&Setting{RepoID: repoID, SettingKey: key})
	return err
}

func SetSettingNoVersion(repoID int64, key, value string) error {
	s, err := GetSettingNoCache(repoID, key)
	if errors.Is(err, util.ErrNotExist) {
		return SetSetting(&Setting{
			RepoID:       repoID,
			SettingKey:   key,
			SettingValue: value,
		})
	}
	if err != nil {
		return err
	}
	s.SettingValue = value
	return SetSetting(s)
}

// SetSetting updates a users' setting for a specific key
func SetSetting(setting *Setting) error {
	if err := upsertSettingValue(setting.RepoID, strings.ToLower(setting.SettingKey), setting.SettingValue, setting.Version); err != nil {
		return err
	}

	setting.Version++

	cc := cache.GetCache()
	if cc != nil {
		return cc.Put(genSettingCacheKey(setting.RepoID, setting.SettingKey), setting.SettingValue, setting_module.CacheService.TTLSeconds())
	}

	return nil
}

func upsertSettingValue(repoID int64, key, value string, version int) error {
	return db.WithTx(db.DefaultContext, func(ctx context.Context) error {
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

		res, err := e.Exec("UPDATE repo_setting SET setting_value=?, version = version+1 WHERE repo_id=? AND setting_key=? AND version=?", value, repoID, key, version)
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
