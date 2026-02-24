// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/log"
)

// Admin settings
var Admin struct {
	DisableRegularOrgCreation   bool
	DefaultEmailNotification    string
	UserDisabledFeatures        container.Set[string]
	ExternalUserDisableFeatures container.Set[string]
}

var validUserFeatures = container.SetOf(
	UserFeatureDeletion,
	UserFeatureManageSSHKeys,
	UserFeatureManageGPGKeys,
	UserFeatureManageMFA,
	UserFeatureManageCredentials,
	UserFeatureChangeUsername,
	UserFeatureChangeFullName,
)

func loadAdminFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("admin")
	Admin.DisableRegularOrgCreation = sec.Key("DISABLE_REGULAR_ORG_CREATION").MustBool(false)
	Admin.DefaultEmailNotification = sec.Key("DEFAULT_EMAIL_NOTIFICATIONS").MustString("enabled")
	Admin.UserDisabledFeatures = container.SetOf(sec.Key("USER_DISABLED_FEATURES").Strings(",")...)
	Admin.ExternalUserDisableFeatures = container.SetOf(sec.Key("EXTERNAL_USER_DISABLE_FEATURES").Strings(",")...).Union(Admin.UserDisabledFeatures)

	for feature := range Admin.UserDisabledFeatures {
		if !validUserFeatures.Contains(feature) {
			log.Warn("USER_DISABLED_FEATURES contains unknown feature %q", feature)
		}
	}
	for feature := range Admin.ExternalUserDisableFeatures {
		if !validUserFeatures.Contains(feature) && !Admin.UserDisabledFeatures.Contains(feature) {
			log.Warn("EXTERNAL_USER_DISABLE_FEATURES contains unknown feature %q", feature)
		}
	}
}

const (
	UserFeatureDeletion          = "deletion"
	UserFeatureManageSSHKeys     = "manage_ssh_keys"
	UserFeatureManageGPGKeys     = "manage_gpg_keys"
	UserFeatureManageMFA         = "manage_mfa"
	UserFeatureManageCredentials = "manage_credentials"
	UserFeatureChangeUsername    = "change_username"
	UserFeatureChangeFullName    = "change_full_name"
)
