// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"strings"

	"code.gitea.io/gitea/modules/container"
)

// userSetting represents user settings
type userSetting struct {
	content container.Set[string]
}

func (s *userSetting) Enabled(module string) bool {
	return !s.content.Contains(strings.ToLower(module))
}

var User userSetting

func loadUserFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("user")
	values := sec.Key("SETTING_DISABLED_MODULES").Strings(",")
	User.content = container.SetOf(values...)
}
