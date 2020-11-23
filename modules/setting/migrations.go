// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"strings"
)

var (
	// Migrations settings
	Migrations = struct {
		MaxAttempts        int
		RetryBackoff       int
		AllowlistedDomains []string
		BlocklistedDomains []string
		AllowLocalNetworks bool
	}{
		MaxAttempts:  3,
		RetryBackoff: 3,
	}
)

func newMigrationsService() {
	sec := Cfg.Section("migrations")
	Migrations.MaxAttempts = sec.Key("MAX_ATTEMPTS").MustInt(Migrations.MaxAttempts)
	Migrations.RetryBackoff = sec.Key("RETRY_BACKOFF").MustInt(Migrations.RetryBackoff)

	Migrations.AllowlistedDomains = sec.Key("ALLOWLISTED_DOMAINS").Strings(",")
	for i := range Migrations.AllowlistedDomains {
		Migrations.AllowlistedDomains[i] = strings.ToLower(Migrations.AllowlistedDomains[i])
	}
	Migrations.BlocklistedDomains = sec.Key("BLOCKLISTED_DOMAINS").Strings(",")
	for i := range Migrations.BlocklistedDomains {
		Migrations.BlocklistedDomains[i] = strings.ToLower(Migrations.BlocklistedDomains[i])
	}

	Migrations.AllowLocalNetworks = sec.Key("ALLOW_LOCALNETWORKS").MustBool(false)
}
