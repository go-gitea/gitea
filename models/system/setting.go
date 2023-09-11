// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package system

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/cache"
	setting_module "code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

	"strk.kbt.io/projects/go/libravatar"
	"xorm.io/builder"
)

// Setting is a key value store of user settings
type Setting struct {
	ID           int64              `xorm:"pk autoincr"`
	SettingKey   string             `xorm:"varchar(255) unique"` // ensure key is always lowercase
	SettingValue string             `xorm:"text"`
	Version      int                `xorm:"version"` // prevent to override
	Created      timeutil.TimeStamp `xorm:"created"`
	Updated      timeutil.TimeStamp `xorm:"updated"`
}

// TableName sets the table name for the settings struct
func (s *Setting) TableName() string {
	return "system_setting"
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

// ErrSettingIsNotExist represents an error that a setting is not exist with special key
type ErrSettingIsNotExist struct {
	Key string
}

// Error implements error
func (err ErrSettingIsNotExist) Error() string {
	return fmt.Sprintf("System setting[%s] is not exist", err.Key)
}

// IsErrSettingIsNotExist return true if err is ErrSettingIsNotExist
func IsErrSettingIsNotExist(err error) bool {
	_, ok := err.(ErrSettingIsNotExist)
	return ok
}

// ErrDataExpired represents an error that update a record which has been updated by another thread
type ErrDataExpired struct {
	Key string
}

// Error implements error
func (err ErrDataExpired) Error() string {
	return fmt.Sprintf("System setting[%s] has been updated by another thread", err.Key)
}

// IsErrDataExpired return true if err is ErrDataExpired
func IsErrDataExpired(err error) bool {
	_, ok := err.(ErrDataExpired)
	return ok
}

// GetSetting returns specific setting without using the cache
func GetSetting(ctx context.Context, key string) (*Setting, error) {
	v, err := GetSettings(ctx, []string{key})
	if err != nil {
		return nil, err
	}
	if len(v) == 0 {
		return nil, ErrSettingIsNotExist{key}
	}
	return v[strings.ToLower(key)], nil
}

const contextCacheKey = "system_setting"

// GetSettingWithCache returns the setting value via the key
func GetSettingWithCache(ctx context.Context, key, defaultVal string) (string, error) {
	return cache.GetWithContextCache(ctx, contextCacheKey, key, func() (string, error) {
		return cache.GetString(genSettingCacheKey(key), func() (string, error) {
			res, err := GetSetting(ctx, key)
			if err != nil {
				if IsErrSettingIsNotExist(err) {
					return defaultVal, nil
				}
				return "", err
			}
			return res.SettingValue, nil
		})
	})
}

// GetSettingBool return bool value of setting,
// none existing keys and errors are ignored and result in false
func GetSettingBool(ctx context.Context, key string, defaultVal bool) (bool, error) {
	s, err := GetSetting(ctx, key)
	switch {
	case err == nil:
		v, _ := strconv.ParseBool(s.SettingValue)
		return v, nil
	case IsErrSettingIsNotExist(err):
		return defaultVal, nil
	default:
		return false, err
	}
}

func GetSettingWithCacheBool(ctx context.Context, key string, defaultVal bool) bool {
	s, _ := GetSettingWithCache(ctx, key, strconv.FormatBool(defaultVal))
	v, _ := strconv.ParseBool(s)
	return v
}

// GetSettings returns specific settings
func GetSettings(ctx context.Context, keys []string) (map[string]*Setting, error) {
	for i := 0; i < len(keys); i++ {
		keys[i] = strings.ToLower(keys[i])
	}
	settings := make([]*Setting, 0, len(keys))
	if err := db.GetEngine(ctx).
		Where(builder.In("setting_key", keys)).
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

// GetAllSettings returns all settings from user
func GetAllSettings(ctx context.Context) (AllSettings, error) {
	settings := make([]*Setting, 0, 5)
	if err := db.GetEngine(ctx).
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
func DeleteSetting(ctx context.Context, setting *Setting) error {
	cache.RemoveContextData(ctx, contextCacheKey, setting.SettingKey)
	cache.Remove(genSettingCacheKey(setting.SettingKey))
	_, err := db.GetEngine(ctx).Delete(setting)
	return err
}

func SetSettingNoVersion(ctx context.Context, key, value string) error {
	s, err := GetSetting(ctx, key)
	if IsErrSettingIsNotExist(err) {
		return SetSetting(ctx, &Setting{
			SettingKey:   key,
			SettingValue: value,
		})
	}
	if err != nil {
		return err
	}
	s.SettingValue = value
	return SetSetting(ctx, s)
}

// SetSetting updates a users' setting for a specific key
func SetSetting(ctx context.Context, setting *Setting) error {
	if err := upsertSettingValue(ctx, strings.ToLower(setting.SettingKey), setting.SettingValue, setting.Version); err != nil {
		return err
	}

	setting.Version++

	cc := cache.GetCache()
	if cc != nil {
		if err := cc.Put(genSettingCacheKey(setting.SettingKey), setting.SettingValue, setting_module.CacheService.TTLSeconds()); err != nil {
			return err
		}
	}
	cache.SetContextData(ctx, contextCacheKey, setting.SettingKey, setting.SettingValue)
	return nil
}

func upsertSettingValue(parentCtx context.Context, key, value string, version int) error {
	return db.WithTx(parentCtx, func(ctx context.Context) error {
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

		res, err := e.Exec("UPDATE system_setting SET setting_value=?, version = version+1 WHERE setting_key=? AND version=?", value, key, version)
		if err != nil {
			return err
		}
		rows, _ := res.RowsAffected()
		if rows > 0 {
			// the existing row is updated, so we can return
			return nil
		}

		// in case the value isn't changed, update would return 0 rows changed, so we need this check
		has, err := e.Exist(&Setting{SettingKey: key})
		if err != nil {
			return err
		}
		if has {
			return ErrDataExpired{Key: key}
		}

		// if no existing row, insert a new row
		_, err = e.Insert(&Setting{SettingKey: key, SettingValue: value})
		return err
	})
}

var (
	GravatarSourceURL *url.URL
	LibravatarService *libravatar.Libravatar
)

func Init(ctx context.Context) error {
	disableGravatar, err := GetSettingBool(ctx, KeyPictureDisableGravatar, setting_module.GetDefaultDisableGravatar())
	if err != nil {
		return err
	}

	enableFederatedAvatar, err := GetSettingBool(ctx, KeyPictureEnableFederatedAvatar, setting_module.GetDefaultEnableFederatedAvatar(disableGravatar))
	if err != nil {
		return err
	}

	if setting_module.OfflineMode {
		if !disableGravatar {
			if err := SetSettingNoVersion(ctx, KeyPictureDisableGravatar, "true"); err != nil {
				return fmt.Errorf("failed to set setting %q: %w", KeyPictureDisableGravatar, err)
			}
		}
		disableGravatar = true

		if enableFederatedAvatar {
			if err := SetSettingNoVersion(ctx, KeyPictureEnableFederatedAvatar, "false"); err != nil {
				return fmt.Errorf("failed to set setting %q: %w", KeyPictureEnableFederatedAvatar, err)
			}
		}
		enableFederatedAvatar = false
	}

	if enableFederatedAvatar || !disableGravatar {
		var err error
		GravatarSourceURL, err = url.Parse(setting_module.GravatarSource)
		if err != nil {
			return fmt.Errorf("failed to parse Gravatar URL(%s): %w", setting_module.GravatarSource, err)
		}
	}

	if GravatarSourceURL != nil && enableFederatedAvatar {
		LibravatarService = libravatar.New()
		if GravatarSourceURL.Scheme == "https" {
			LibravatarService.SetUseHTTPS(true)
			LibravatarService.SetSecureFallbackHost(GravatarSourceURL.Host)
		} else {
			LibravatarService.SetUseHTTPS(false)
			LibravatarService.SetFallbackHost(GravatarSourceURL.Host)
		}
	}
	return nil
}
