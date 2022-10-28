// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package system

import (
	"fmt"

	"code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/cache"
)

func genUserKey(userID int64, key string) string {
	return fmt.Sprintf("user_%d.setting.%s", userID, key)
}

// GetUserSetting returns the user setting value via the key
func GetUserSetting(userID int64, key string) (string, error) {
	return cache.GetString(genUserKey(userID, key), func() (string, error) {
		res, err := user.GetSetting(userID, key)
		if err != nil {
			return "", err
		}
		return res.SettingValue, nil
	})
}

// SetUserSetting sets the user setting value
func SetUserSetting(userID int64, key, value string) error {
	cache.Remove(genUserKey(userID, key))

	return user.SetUserSetting(userID, key, value)
}
