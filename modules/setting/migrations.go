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
	// the batched GraphQL fast path (see services/migrations/github_graphql.go),
	// cutting rate-limit pressure on large repositories. Off by default; set
	// [migrations] USE_GRAPHQL_FOR_MIRROR = true to enable (e.g. for A/B timing).
	// SyncReactionsForMirror opts metadata mirror syncs into fetching issue/PR/comment
	// reactions too. Off by default because reactions are an N+1 call storm (the lowest
	// -value, heaviest metadata for a read-only mirror); set
	// [migrations] SYNC_REACTIONS_FOR_MIRROR = true to include them.
	SyncReactionsForMirror bool
}{
	MaxAttempts:  3,
	RetryBackoff: 3,
}

func loadMigrationsFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("migrations")
	Migrations.MaxAttempts = sec.Key("MAX_ATTEMPTS").MustInt(Migrations.MaxAttempts)
	Migrations.RetryBackoff = sec.Key("RETRY_BACKOFF").MustInt(Migrations.RetryBackoff)

	Migrations.AllowedDomains = sec.Key("ALLOWED_DOMAINS").MustString("")
	Migrations.BlockedDomains = sec.Key("BLOCKED_DOMAINS").MustString("")
	Migrations.AllowLocalNetworks = sec.Key("ALLOW_LOCALNETWORKS").MustBool(false)
	Migrations.SkipTLSVerify = sec.Key("SKIP_TLS_VERIFY").MustBool(false)
	Migrations.SyncReactionsForMirror = sec.Key("SYNC_REACTIONS_FOR_MIRROR").MustBool(false)
}
