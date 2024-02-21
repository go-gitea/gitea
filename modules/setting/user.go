// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"code.gitea.io/gitea/modules/container"
)

const (
	UserDeletionKey = "deletion"
)

// userSetting represents user settings
type userSetting struct {
	disabledModules container.Set[string]
}

func (s *userSetting) Enabled(module string) bool {
	return !s.disabledModules.Contains(module)
}

var User userSetting

func loadUserFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("user")
	values := sec.Key("SETTING_DISABLED_MODULES").Strings(",")
	User.disabledModules = container.SetOf(values...)
}
