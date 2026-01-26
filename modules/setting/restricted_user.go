// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

var RestrictedUser = struct {
	AllowEditDueDate bool
}{}

func loadRestrictedUserFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("restricted_user")

	RestrictedUser.AllowEditDueDate = sec.Key("ALLOW_EDIT_DUE_DATE").MustBool(false)
}
