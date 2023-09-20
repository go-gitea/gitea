// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

// Admin settings
var Admin struct {
	DisableRegularOrgCreation      bool
	DefaultEmailNotification       string
	SendNotificationEmailOnNewUser bool
}

func loadAdminFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "admin", &Admin)
	sec := rootCfg.Section("admin")
	Admin.DefaultEmailNotification = sec.Key("DEFAULT_EMAIL_NOTIFICATIONS").MustString("enabled")
}
