// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package system

import (
	"code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/cache"
)

func genKey(key string) string {
	return "system.setting." + key
}

// GetSetting returns the setting value via the key
func GetSetting(key string) (string, error) {
	return cache.GetString(genKey(key), func() (string, error) {
		res, err := system.GetSetting(key)
		if err != nil {
			return "", err
		}
		return res.SettingValue, nil
	})
}

// SetSetting sets the setting value
func SetSetting(key, value string) error {
	cache.Remove(genKey(key))

	return system.SetSetting(&system.Setting{
		SettingKey:   key,
		SettingValue: value,
	})
}
