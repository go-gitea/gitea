// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"strings"

	"code.gitea.io/gitea/modules/log"
)

// userSetting represents user settings
type userSetting struct {
	SettingDisabledModules []string
}

func (s *userSetting) Enabled(module string) bool {
	for _, m := range s.SettingDisabledModules {
		if strings.EqualFold(m, module) {
			return false
		}
	}
	return true
}

var User userSetting

func newUserSetting() {
	sec := Cfg.Section("user")
	if err := sec.MapTo(&User); err != nil {
		log.Fatal("user setting mapping failed: %v", err)
	}
}
