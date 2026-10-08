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
	// SSHHostKeyChecking controls StrictHostKeyChecking for SSH migrations/mirrors:
	// "accept-new" (default, trust on first use, reject changed keys), "yes" (strict,
	// host must already be known) or "no" (disable verification).
	SSHHostKeyChecking string
	// SSHCommand is the ssh executable used for SSH migrations/mirrors. Defaults to
	// "ssh"; set an absolute path when ssh is not on PATH (e.g. on Windows).
	SSHCommand string
}{
	MaxAttempts:        3,
	RetryBackoff:       3,
	EgressMode:         "lax",
	SSHHostKeyChecking: "accept-new",
	SSHCommand:         "ssh",
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
	Migrations.SSHHostKeyChecking = sec.Key("SSH_HOST_KEY_CHECKING").In("accept-new", []string{"accept-new", "yes", "no"})
	Migrations.SSHCommand = sec.Key("SSH_COMMAND").MustString("ssh")
}
