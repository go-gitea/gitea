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
	UseGraphQL:   true,
}

func loadMigrationsFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("migrations")
	Migrations.MaxAttempts = sec.Key("MAX_ATTEMPTS").MustInt(Migrations.MaxAttempts)
	Migrations.RetryBackoff = sec.Key("RETRY_BACKOFF").MustInt(Migrations.RetryBackoff)

	Migrations.AllowedDomains = sec.Key("ALLOWED_DOMAINS").MustString("")
	Migrations.BlockedDomains = sec.Key("BLOCKED_DOMAINS").MustString("")
	Migrations.AllowLocalNetworks = sec.Key("ALLOW_LOCALNETWORKS").MustBool(false)
	Migrations.SkipTLSVerify = sec.Key("SKIP_TLS_VERIFY").MustBool(false)
	Migrations.UseGraphQL = sec.Key("USE_GRAPHQL").MustBool(true)
}
