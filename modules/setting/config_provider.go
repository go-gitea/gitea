// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"code.gitea.io/gitea/modules/log"

	ini "gopkg.in/ini.v1"
)

// ConfigProvider represents a config provider
type ConfigProvider interface {
	Section(section string) *ini.Section
	NewSection(name string) (*ini.Section, error)
	GetSection(name string) (*ini.Section, error)
}

// a file is an implementation ConfigProvider and other implementations are possible, i.e. from docker, k8s, …
var _ ConfigProvider = &ini.File{}

func mustMapSetting(rootCfg ConfigProvider, sectionName string, setting interface{}) {
	if err := rootCfg.Section(sectionName).MapTo(setting); err != nil {
		log.Fatal("Failed to map %s settings: %v", sectionName, err)
	}
}

func deprecatedSetting(rootCfg ConfigProvider, oldSection, oldKey, newSection, newKey string) {
	if rootCfg.Section(oldSection).HasKey(oldKey) {
		log.Error("Deprecated fallback `[%s]` `%s` present. Use `[%s]` `%s` instead. This fallback will be removed in v1.19.0", oldSection, oldKey, newSection, newKey)
	}
}

// deprecatedSettingDB add a hint that the configuration has been moved to database but still kept in app.ini
func deprecatedSettingDB(rootCfg ConfigProvider, oldSection, oldKey string) {
	if rootCfg.Section(oldSection).HasKey(oldKey) {
		log.Error("Deprecated `[%s]` `%s` present which has been copied to database table sys_setting", oldSection, oldKey)
	}
}
