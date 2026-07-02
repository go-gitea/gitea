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
	SSHHostKeyChecking: "accept-new",
	SSHCommand:         "ssh",
}

func loadMigrationsFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("migrations")
	Migrations.MaxAttempts = sec.Key("MAX_ATTEMPTS").MustInt(Migrations.MaxAttempts)
	Migrations.RetryBackoff = sec.Key("RETRY_BACKOFF").MustInt(Migrations.RetryBackoff)

	Migrations.AllowedDomains = sec.Key("ALLOWED_DOMAINS").MustString("")
	Migrations.BlockedDomains = sec.Key("BLOCKED_DOMAINS").MustString("")
	Migrations.AllowLocalNetworks = sec.Key("ALLOW_LOCALNETWORKS").MustBool(false)
	Migrations.SkipTLSVerify = sec.Key("SKIP_TLS_VERIFY").MustBool(false)
	Migrations.SSHHostKeyChecking = sec.Key("SSH_HOST_KEY_CHECKING").In("accept-new", []string{"accept-new", "yes", "no"})
	Migrations.SSHCommand = sec.Key("SSH_COMMAND").MustString("ssh")
}
