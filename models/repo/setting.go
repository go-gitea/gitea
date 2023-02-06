// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"code.gitea.io/gitea/models/db"
)

type Setting db.ResourceSetting

const repoSettingTableName = "repo_setting"

// TableName sets the table name for the settings struct
func (s *Setting) TableName() string {
	return repoSettingTableName
}

func init() {
	db.RegisterModel(new(Setting))
}

func SetSetting(s *Setting) error {
	return db.SetSetting(db.DefaultContext, repoSettingTableName, (*db.ResourceSetting)(s))
}

func GetSettings(repoID int64, keys []string) (map[string]*Setting, error) {
	settings := make(map[string]*Setting)
	resourceSettings, err := db.GetSettings(db.DefaultContext, repoSettingTableName, repoID, keys)
	if err != nil {
		return nil, err
	}
	for key, setting := range resourceSettings {
		settings[key] = (*Setting)(setting)
	}
	return settings, nil
}

func GetSetting(repoID int64, key string) (string, error) {
	return db.GetSetting(db.DefaultContext, repoSettingTableName, repoID, key)
}

func DeleteSetting(repoID int64, key string) error {
	return db.DeleteSetting(db.DefaultContext, repoSettingTableName, repoID, key)
}

func GetAllSettings(repoID int64) (map[string]*Setting, error) {
	settings := make(map[string]*Setting)
	resourceSettings, err := db.GetAllSettings(db.DefaultContext, repoSettingTableName, repoID)
	if err != nil {
		return nil, err
	}
	for key, setting := range resourceSettings {
		settings[key] = (*Setting)(setting)
	}
	return settings, nil
}
