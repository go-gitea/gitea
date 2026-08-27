// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

// User settings
var User struct {
	MaxSSHKeysPerUser int
	MaxGPGKeysPerUser int
}

func loadUserFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("user")
	User.MaxSSHKeysPerUser = sec.Key("MAX_SSH_KEYS_PER_USER").MustInt(8)
	User.MaxGPGKeysPerUser = sec.Key("MAX_GPG_KEYS_PER_USER").MustInt(8)
}
