// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import "code.gitea.io/gitea/modules/setting/base"

// Admin settings
var Admin struct {
	DisableRegularOrgCreation bool
	DefaultEmailNotification  string
}

func loadAdminFrom(rootCfg base.ConfigProvider) {
	base.MustMapSetting(rootCfg, "admin", &Admin)
	sec := rootCfg.Section("admin")
	Admin.DefaultEmailNotification = sec.Key("DEFAULT_EMAIL_NOTIFICATIONS").MustString("enabled")
}
