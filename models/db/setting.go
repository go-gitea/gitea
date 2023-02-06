// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/cache"
	setting_module "code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

type ResourceSetting struct {
	ID           int64              `xorm:"pk autoincr"`
	GroupID      int64              `xorm:"index unique(key_repoid)"`               // to load all of some group's settings
	SettingKey   string             `xorm:"varchar(255) index unique(key_groupid)"` // ensure key is always lowercase
	SettingValue string             `xorm:"text"`
	Version      int                `xorm:"version"` // prevent to override
	Created      timeutil.TimeStamp `xorm:"created"`
	Updated      timeutil.TimeStamp `xorm:"updated"`
}

func (s *ResourceSetting) AsBool() bool {
	if s == nil {
		return false
	}

	b, _ := strconv.ParseBool(s.SettingValue)
	return b
}

// GetSettings returns specific settings
func GetSettings(ctx context.Context, tableName string, groupID int64, keys []string) (map[string]*ResourceSetting, error) {
	for i := 0; i < len(keys); i++ {
		keys[i] = strings.ToLower(keys[i])
	}
	settings := make([]*ResourceSetting, 0, len(keys))
	if err := GetEngine(ctx).
		Table(tableName).
		Where("group_id=?", groupID).
		And(builder.In("setting_key", keys)).
		Find(&settings); err != nil {
		return nil, err
	}
	settingsMap := make(map[string]*ResourceSetting)
	for _, s := range settings {
		settingsMap[s.SettingKey] = s
	}
	return settingsMap, nil
}

func ValidateSettingKey(key string) error {
	if len(key) == 0 {
		return fmt.Errorf("setting key must be set")
	}
	if strings.ToLower(key) != key {
		return fmt.Errorf("setting key should be lowercase")
	}
	return nil
}

// genSettingCacheKey returns the cache key for some configuration
func genSettingCacheKey(tableName string, groupID int64, key string) string {
	return fmt.Sprintf("%s.setting.%d.%s", tableName, groupID, key)
}

// GetSettingNoCache returns specific setting without using the cache
func GetSettingNoCache(ctx context.Context, tableName string, groupID int64, key string) (*ResourceSetting, error) {
	v, err := GetSettings(ctx, tableName, groupID, []string{key})
	if err != nil {
		return nil, err
	}
	if len(v) == 0 {
		return nil, fmt.Errorf("%s[%d] setting[%s]: %w", tableName, groupID, key, util.ErrNotExist)
	}
	return v[strings.ToLower(key)], nil
}

// GetSetting returns the setting value via the key
func GetSetting(ctx context.Context, tableName string, groupID int64, key string) (string, error) {
	if err := ValidateSettingKey(key); err != nil {
		return "", err
	}
	return cache.GetString(genSettingCacheKey(tableName, groupID, key), func() (string, error) {
		res, err := GetSettingNoCache(ctx, tableName, groupID, key)
		if err != nil {
			return "", err
		}
		return res.SettingValue, nil
	})
}

// GetSettingBool return bool value of setting,
// none existing keys and errors are ignored and result in false
func GetSettingBool(ctx context.Context, tableName string, groupID int64, key string) bool {
	s, _ := GetSetting(ctx, tableName, groupID, key)
	v, _ := strconv.ParseBool(s)
	return v
}

type AllSettings map[string]*ResourceSetting

func (settings AllSettings) Get(key string) ResourceSetting {
	if v, ok := settings[strings.ToLower(key)]; ok {
		return *v
	}
	return ResourceSetting{}
}

func (settings AllSettings) GetBool(key string) bool {
	b, _ := strconv.ParseBool(settings.Get(key).SettingValue)
	return b
}

func (settings AllSettings) GetVersion(key string) int {
	return settings.Get(key).Version
}

// GetAllSettings returns all settings from repo
func GetAllSettings(ctx context.Context, tableName string, groupID int64) (AllSettings, error) {
	settings := make([]*ResourceSetting, 0, 5)
	if err := GetEngine(ctx).
		Table(tableName).
		Where("group_id=?", groupID).
		Find(&settings); err != nil {
		return nil, err
	}
	settingsMap := make(map[string]*ResourceSetting)
	for _, s := range settings {
		settingsMap[s.SettingKey] = s
	}
	return settingsMap, nil
}

// DeleteSetting deletes a specific setting for a repo
func DeleteSetting(ctx context.Context, tableName string, groupID int64, key string) error {
	if err := ValidateSettingKey(key); err != nil {
		return err
	}
	cache.Remove(genSettingCacheKey(tableName, groupID, key))
	_, err := GetEngine(ctx).Table(tableName).Delete(&ResourceSetting{GroupID: groupID, SettingKey: key})
	return err
}

func SetSettingNoVersion(ctx context.Context, tableName string, groupID int64, key, value string) error {
	s, err := GetSettingNoCache(ctx, tableName, groupID, key)
	if errors.Is(err, util.ErrNotExist) {
		return SetSetting(ctx, tableName, &ResourceSetting{
			GroupID:      groupID,
			SettingKey:   key,
			SettingValue: value,
		})
	}
	if err != nil {
		return err
	}
	s.SettingValue = value
	return SetSetting(ctx, tableName, s)
}

// SetSetting updates a users' setting for a specific key
func SetSetting(ctx context.Context, tableName string, setting *ResourceSetting) error {
	if err := upsertSettingValue(ctx, tableName, setting.GroupID, strings.ToLower(setting.SettingKey), setting.SettingValue, setting.Version); err != nil {
		return err
	}

	setting.Version++

	cc := cache.GetCache()
	if cc != nil {
		return cc.Put(genSettingCacheKey(tableName, setting.GroupID, setting.SettingKey), setting.SettingValue, setting_module.CacheService.TTLSeconds())
	}

	return nil
}

func upsertSettingValue(ctx context.Context, tableName string, groupID int64, key, value string, version int) error {
	return WithTx(ctx, func(ctx context.Context) error {
		e := GetEngine(ctx)

		// here we use a general method to do a safe upsert for different databases (and most transaction levels)
		// 1. try to UPDATE the record and acquire the transaction write lock
		//    if UPDATE returns non-zero rows are changed, OK, the setting is saved correctly
		//    if UPDATE returns "0 rows changed", two possibilities: (a) record doesn't exist  (b) value is not changed
		// 2. do a SELECT to check if the row exists or not (we already have the transaction lock)
		// 3. if the row doesn't exist, do an INSERT (we are still protected by the transaction lock, so it's safe)
		//
		// to optimize the SELECT in step 2, we can use an extra column like `revision=revision+1`
		//    to make sure the UPDATE always returns a non-zero value for existing (unchanged) records.

		res, err := e.Exec(fmt.Sprintf("UPDATE %s SET setting_value=?, version = version+1 WHERE group_id=? AND setting_key=? AND version=?", tableName), value, groupID, key, version)
		if err != nil {
			return err
		}
		rows, _ := res.RowsAffected()
		if rows > 0 {
			// the existing row is updated, so we can return
			return nil
		}

		// in case the value isn't changed, update would return 0 rows changed, so we need this check
		has, err := e.Table(tableName).Exist(&ResourceSetting{GroupID: groupID, SettingKey: key})
		if err != nil {
			return err
		}
		if has {
			return nil
		}

		// if no existing row, insert a new row
		_, err = e.Table(tableName).Insert(&ResourceSetting{GroupID: groupID, SettingKey: key, SettingValue: value})
		return err
	})
}
