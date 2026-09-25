// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

// Migrations settings
var Migrations = struct {
	MaxAttempts     int
	RetryBackoff    int
	AllowedHostList string
	BlockedHostList string
	SkipTLSVerify   bool
}{
	MaxAttempts:     3,
	RetryBackoff:    3,
	AllowedHostList: "external",
}

func loadMigrationsFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("migrations")
	Migrations.MaxAttempts = sec.Key("MAX_ATTEMPTS").MustInt(Migrations.MaxAttempts)
	Migrations.RetryBackoff = sec.Key("RETRY_BACKOFF").MustInt(Migrations.RetryBackoff)

	deprecatedSetting(rootCfg, "migrations", "ALLOWED_DOMAINS", "migrations", "ALLOWED_HOST_LIST", "v28.0.0")
	deprecatedSetting(rootCfg, "migrations", "BLOCKED_DOMAINS", "migrations", "BLOCKED_HOST_LIST", "v28.0.0")
	deprecatedSetting(rootCfg, "migrations", "ALLOW_LOCALNETWORKS", "migrations", "ALLOWED_HOST_LIST", "v28.0.0")
	Migrations.AllowedHostList = ConfigSectionKeyString(sec, "ALLOWED_HOST_LIST", ConfigSectionKeyString(sec, "ALLOWED_DOMAINS", "external"))
	Migrations.BlockedHostList = ConfigSectionKeyString(sec, "BLOCKED_HOST_LIST", ConfigSectionKeyString(sec, "BLOCKED_DOMAINS"))
	if ConfigSectionKeyBool(sec, "ALLOW_LOCALNETWORKS") {
		Migrations.AllowedHostList += ",private,loopback"
	}

	Migrations.SkipTLSVerify = sec.Key("SKIP_TLS_VERIFY").MustBool(false)
}
