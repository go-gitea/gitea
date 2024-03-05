// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
)

// Admin settings
var Admin struct {
	DisableRegularOrgCreation   bool
	DefaultEmailNotification    string
	userDisabledFeatures        container.Set[string]
	ExternalUserDisableFeatures bool
}

func loadAdminFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("admin")
	Admin.DisableRegularOrgCreation = sec.Key("DISABLE_REGULAR_ORG_CREATION").MustBool(false)
	Admin.DefaultEmailNotification = sec.Key("DEFAULT_EMAIL_NOTIFICATIONS").MustString("enabled")
	Admin.userDisabledFeatures = container.SetOf(sec.Key("USER_DISABLED_FEATURES").Strings(",")...)
	Admin.ExternalUserDisableFeatures = sec.Key("EXTERNAL_USER_DISABLE_FEATURES").MustBool(false)
}

const (
	UserFeatureDeletion      = "deletion"
	UserFeatureManageSSHKeys = "manage_ssh_keys"
	UserFeatureManageGPGKeys = "manage_gpg_keys"
)

var defaultSet = container.SetOf(
	UserFeatureDeletion,
	UserFeatureManageSSHKeys,
	UserFeatureManageGPGKeys)

// UserFeatureDisabled checks if a user feature is disabled
func UserFeatureDisabled(feature string) bool {
	return Admin.userDisabledFeatures.Contains(feature)
}

// UserFeatureDisabledWithLoginType checks if a user feature is disabled, taking into account the login type of the
// user if applicable
func UserFeatureDisabledWithLoginType(user *user_model.User, feature string) bool {
	return Admin.ExternalUserDisableFeatures && user.LoginType > auth.Plain || UserFeatureDisabled(feature)
}

// UserDisabledFeaturesWithLoginType returns the set of user features disabled, taking into account the login type
// of the user if applicable
func UserDisabledFeaturesWithLoginType(user *user_model.User) *container.Set[string] {
	if Admin.ExternalUserDisableFeatures && user.LoginType > auth.Plain {
		return &defaultSet
	}
	return &Admin.userDisabledFeatures
}
