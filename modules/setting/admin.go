// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"code.gitea.io/gitea/modules/container"
)

// Admin settings
var Admin struct {
	DisableRegularOrgCreation      bool
	DefaultEmailNotification       string
	UserDisabledFeatures           container.Set[string]
	ExternalUserDisableAllFeatures bool
}

func loadAdminFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("admin")
	Admin.DisableRegularOrgCreation = sec.Key("DISABLE_REGULAR_ORG_CREATION").MustBool(false)
	Admin.DefaultEmailNotification = sec.Key("DEFAULT_EMAIL_NOTIFICATIONS").MustString("enabled")
	Admin.UserDisabledFeatures = container.SetOf(sec.Key("USER_DISABLED_FEATURES").Strings(",")...)
	Admin.ExternalUserDisableAllFeatures = sec.Key("EXTERNAL_USER_DISABLE_ALL_FEATURES").MustBool(false)
}

const (
	UserFeatureDeletion      = "deletion"
	UserFeatureManageSSHKeys = "manage_ssh_keys"
	UserFeatureManageGPGKeys = "manage_gpg_keys"
)

var DefaultUserFeatureSet = container.SetOf(
	UserFeatureDeletion,
	UserFeatureManageSSHKeys,
	UserFeatureManageGPGKeys)
