// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import "code.gitea.io/gitea/modules/container"

// Admin settings
var Admin struct {
	DisableRegularOrgCreation bool
	DefaultEmailNotification  string
	UserDisabledModules       container.Set[string]
}

func loadAdminFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "admin", &Admin)
	sec := rootCfg.Section("admin")
	Admin.DefaultEmailNotification = sec.Key("DEFAULT_EMAIL_NOTIFICATIONS").MustString("enabled")

	values := sec.Key("USER_SETTING_DISABLED_MODULES").Strings(",")
	Admin.UserDisabledModules = container.SetOf(values...)
}

const (
	UserDeletionKey = "deletion"
)
