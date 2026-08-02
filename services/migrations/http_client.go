// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"crypto/tls"
	"net/http"

	"gitea.dev/modules/hostmatcher"
	"gitea.dev/modules/proxy"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

// migrationHTTPClient is the shared migration client. Callers that would otherwise build a client per
// request use it (via getMigrationHTTPClient) so a single connection pool is reused across downloads —
// e.g. many release assets from the same host — instead of a fresh pool and TLS handshake each time. It
// is built lazily on first use and reset by Init whenever the allow/block lists change; OnceValue keeps
// concurrent callers sharing a single client instead of racing to create their own.
var migrationHTTPClient = util.OnceValue[*http.Client]{Func: newMigrationHTTPClient}

// newMigrationHTTPClient returns a HTTP client for migration
func newMigrationHTTPClient() *http.Client {
	return &http.Client{
		Transport: NewMigrationHTTPTransport(),
	}
}

// getMigrationHTTPClient returns the shared migration client, building it on first use so no request
// escapes the SSRF-validated transport even before Init has run.
func getMigrationHTTPClient() *http.Client {
	return migrationHTTPClient.Value()
}

// NewMigrationHTTPTransport returns a HTTP transport for migration. The target is validated against the
// allow/block lists on both the direct-dial and proxy paths, so a configured proxy cannot be used to
// reach an otherwise-forbidden target (SSRF).
func NewMigrationHTTPTransport() *http.Transport {
	return hostmatcher.NewHTTPTransport("migration", allowList, blockList, proxy.Proxy(), setting.Proxy.ProxyURLFixed,
		&tls.Config{InsecureSkipVerify: setting.Migrations.SkipTLSVerify})
}
