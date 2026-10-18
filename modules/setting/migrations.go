// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"strings"

	"gitea.dev/modules/log"
)

// Migrations settings
var Migrations = struct {
	MaxAttempts           int
	RetryBackoff          int
	EgressMode            string
	AllowedHostList       string
	AllowedInternalRanges string
	BlockedHostList       string
	SkipTLSVerify         bool
}{
	MaxAttempts:  3,
	RetryBackoff: 3,
	EgressMode:   "lax",
}

func loadMigrationsFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("migrations")
	Migrations.MaxAttempts = sec.Key("MAX_ATTEMPTS").MustInt(Migrations.MaxAttempts)
	Migrations.RetryBackoff = sec.Key("RETRY_BACKOFF").MustInt(Migrations.RetryBackoff)

	egressModeSet := sec.HasKey("EGRESS_MODE")
	Migrations.EgressMode = normalizePolicyMode(sec.Key("EGRESS_MODE").String())

	deprecatedSetting(rootCfg, "migrations", "ALLOWED_DOMAINS", "migrations", "ALLOWED_HOST_LIST")
	deprecatedSetting(rootCfg, "migrations", "BLOCKED_DOMAINS", "migrations", "BLOCKED_HOST_LIST")
	deprecatedSetting(rootCfg, "migrations", "ALLOW_LOCALNETWORKS", "migrations", "ALLOWED_HOST_LIST")
	Migrations.AllowedHostList = ConfigSectionKeyString(sec, "ALLOWED_HOST_LIST")
	allowKey := "[migrations] ALLOWED_HOST_LIST"
	if Migrations.AllowedHostList == "" {
		allowKey = "[migrations] ALLOWED_DOMAINS/ALLOW_LOCALNETWORKS"
		var hosts []string
		allowedDomains := ConfigSectionKeyString(sec, "ALLOWED_DOMAINS")
		for host := range strings.SplitSeq(allowedDomains, ",") {
			if host = strings.TrimSpace(host); host != "" {
				hosts = append(hosts, host+":*")
			}
		}
		if len(hosts) > 0 && !egressModeSet {
			Migrations.EgressMode = "strict" // ALLOWED_DOMAINS allowed only its hosts, on any port
			LogStartupProblem(1, log.WARN, "[migrations] EGRESS_MODE is not set; defaulting to strict because ALLOWED_DOMAINS is set (the mode also applies to git operations) — set EGRESS_MODE explicitly to choose")
		}
		if allowedDomains != "" && Migrations.EgressMode == "lax" {
			LogStartupProblem(1, log.WARN, "[migrations] ALLOWED_DOMAINS with EGRESS_MODE = lax only restricts non-public targets, migrations from any public host are allowed — the old ALLOWED_DOMAINS allowed only the listed hosts")
		}
		if ConfigSectionKeyBool(sec, "ALLOW_LOCALNETWORKS") {
			hosts = append(hosts, "private:*", "loopback:*")
		}
		Migrations.AllowedHostList = strings.Join(hosts, ",")
	}
	Migrations.BlockedHostList = ConfigSectionKeyString(sec, "BLOCKED_HOST_LIST", ConfigSectionKeyString(sec, "BLOCKED_DOMAINS"))
	blockKey := "[migrations] BLOCKED_HOST_LIST"
	if !sec.HasKey("BLOCKED_HOST_LIST") {
		blockKey = "[migrations] BLOCKED_DOMAINS"
	}
	checkHostList(allowKey, Migrations.AllowedHostList, false)
	checkHostList(blockKey, Migrations.BlockedHostList, true)
	Migrations.AllowedInternalRanges = ConfigSectionKeyString(sec, "ALLOWED_INTERNAL_RANGES")
	checkInternalRanges("[migrations] ALLOWED_INTERNAL_RANGES", Migrations.AllowedInternalRanges, Migrations.AllowedHostList, allowKey)

	Migrations.SkipTLSVerify = sec.Key("SKIP_TLS_VERIFY").MustBool(false)
}
