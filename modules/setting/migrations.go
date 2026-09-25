// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

// Migrations settings
var Migrations = struct {
	MaxAttempts        int
	RetryBackoff       int
	AllowedHostList    string
	DeniedHostList     string
	AllowLocalNetworks bool
	SkipTLSVerify      bool
}{
	MaxAttempts:  3,
	RetryBackoff: 3,
}

func loadMigrationsFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("migrations")
	Migrations.MaxAttempts = sec.Key("MAX_ATTEMPTS").MustInt(Migrations.MaxAttempts)
	Migrations.RetryBackoff = sec.Key("RETRY_BACKOFF").MustInt(Migrations.RetryBackoff)

	deprecatedSetting(rootCfg, "migrations", "ALLOWED_DOMAINS", "migrations", "ALLOWED_HOST_LIST", "v28.0.0")
	deprecatedSetting(rootCfg, "migrations", "BLOCKED_DOMAINS", "migrations", "BLOCKED_HOST_LIST", "v28.0.0")
	Migrations.AllowedHostList = sec.Key("ALLOWED_HOST_LIST").MustString("")
	Migrations.AllowedHostList = sec.Key("ALLOWED_DOMAINS").MustString(Migrations.AllowedHostList)
	Migrations.DeniedHostList = sec.Key("BLOCKED_HOST_LIST").MustString("")
	Migrations.DeniedHostList = sec.Key("BLOCKED_DOMAINS").MustString(Migrations.DeniedHostList)

	deprecatedSetting(rootCfg, "migrations", "ALLOW_LOCALNETWORKS", "migrations", "ALLOWED_HOST_LIST", "v28.0.0")
	Migrations.AllowLocalNetworks = sec.Key("ALLOW_LOCALNETWORKS").MustBool(false)

	Migrations.SkipTLSVerify = sec.Key("SKIP_TLS_VERIFY").MustBool(false)
}
