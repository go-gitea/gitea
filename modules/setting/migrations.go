// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import "strings"

// Migrations settings
var Migrations = struct {
	MaxAttempts     int
	RetryBackoff    int
	EgressMode      string
	AllowedHostList string
	BlockedHostList string
	SkipTLSVerify   bool
	// UseGraphQL routes GitHub migrations through the batched GraphQL fast path
	// (see services/migrations/github_graphql.go), which fetches an issue or pull
	// request together with its comments, reviews and reactions in one request
	// instead of one request per entity. On by default; [migrations] USE_GRAPHQL
	// = false falls back to the REST downloader (e.g. for a server or token that
	// cannot use the GraphQL API). Non-GitHub migrations are unaffected.
	UseGraphQL bool
}{
	MaxAttempts:  3,
	RetryBackoff: 3,
	EgressMode:   "lax",
	UseGraphQL:   true,
}

func loadMigrationsFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("migrations")
	Migrations.MaxAttempts = sec.Key("MAX_ATTEMPTS").MustInt(Migrations.MaxAttempts)
	Migrations.RetryBackoff = sec.Key("RETRY_BACKOFF").MustInt(Migrations.RetryBackoff)

	egressModeSet := sec.HasKey("EGRESS_MODE")
	Migrations.EgressMode = normalizePolicyMode(sec.Key("EGRESS_MODE").String())

	deprecatedSetting(rootCfg, "migrations", "ALLOWED_DOMAINS", "migrations", "ALLOWED_HOST_LIST", "v28.0.0")
	deprecatedSetting(rootCfg, "migrations", "BLOCKED_DOMAINS", "migrations", "BLOCKED_HOST_LIST", "v28.0.0")
	deprecatedSetting(rootCfg, "migrations", "ALLOW_LOCALNETWORKS", "migrations", "ALLOWED_HOST_LIST", "v28.0.0")
	Migrations.AllowedHostList = ConfigSectionKeyString(sec, "ALLOWED_HOST_LIST")
	if Migrations.AllowedHostList == "" {
		var hosts []string
		for host := range strings.SplitSeq(ConfigSectionKeyString(sec, "ALLOWED_DOMAINS"), ",") {
			if host = strings.TrimSpace(host); host != "" {
				hosts = append(hosts, host+":*")
			}
		}
		if len(hosts) > 0 && !egressModeSet {
			Migrations.EgressMode = "strict" // ALLOWED_DOMAINS allowed only its hosts, on any port
		}
		if ConfigSectionKeyBool(sec, "ALLOW_LOCALNETWORKS") {
			hosts = append(hosts, "private:*", "loopback:*")
		}
		Migrations.AllowedHostList = strings.Join(hosts, ",")
	}
	Migrations.BlockedHostList = ConfigSectionKeyString(sec, "BLOCKED_HOST_LIST", ConfigSectionKeyString(sec, "BLOCKED_DOMAINS"))
	checkHostList("[migrations] ALLOWED_HOST_LIST", Migrations.AllowedHostList, false)
	checkHostList("[migrations] BLOCKED_HOST_LIST", Migrations.BlockedHostList, true)

	Migrations.SkipTLSVerify = sec.Key("SKIP_TLS_VERIFY").MustBool(false)
	Migrations.UseGraphQL = sec.Key("USE_GRAPHQL").MustBool(true)
}
