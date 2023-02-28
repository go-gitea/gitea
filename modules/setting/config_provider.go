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
