// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"code.gitea.io/gitea/modules/log"

	ini "gopkg.in/ini.v1"
)

type Config interface {
	Section(section string) *ini.Section
	NewSection(name string) (*ini.Section, error)
	GetSection(name string) (*ini.Section, error)
}

func mustMapSetting(rootCfg Config, sectionName string, setting interface{}) {
	if err := rootCfg.Section(sectionName).MapTo(setting); err != nil {
		log.Fatal("Failed to map %s settings: %v", sectionName, err)
	}
}
