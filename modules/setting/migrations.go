// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

// Migrations settings
var Migrations = struct {
	MaxAttempts        int
	RetryBackoff       int
	AllowedDomains     string
	BlockedDomains     string
	AllowLocalNetworks bool
	SkipTLSVerify      bool
}{
	MaxAttempts:  3,
	RetryBackoff: 3,
}

func newMigrationsService() {
	sec := Cfg.Section("migrations")
	Migrations.MaxAttempts = sec.Key("MAX_ATTEMPTS").MustInt(Migrations.MaxAttempts)
	Migrations.RetryBackoff = sec.Key("RETRY_BACKOFF").MustInt(Migrations.RetryBackoff)

	Migrations.AllowedDomains = sec.Key("ALLOWED_DOMAINS").MustString("")
	Migrations.BlockedDomains = sec.Key("BLOCKED_DOMAINS").MustString("")
	Migrations.AllowLocalNetworks = sec.Key("ALLOW_LOCALNETWORKS").MustBool(false)
	Migrations.SkipTLSVerify = sec.Key("SKIP_TLS_VERIFY").MustBool(false)
}
