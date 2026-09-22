// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"crypto/tls"
	"net/http"
	"sync"

	"gitea.dev/modules/egress"
	"gitea.dev/modules/setting"
)

// migrationHTTPClient is the shared migration client, built once from the policy the git proxy
// enforces. Callers reuse it (via getMigrationHTTPClient) to share one connection pool across
// downloads — e.g. many release assets from the same host — instead of building a fresh pool and
// TLS handshake each time.
var migrationHTTPClient = sync.OnceValue(newMigrationHTTPClient)

// newMigrationHTTPClient returns a HTTP client for migration
func newMigrationHTTPClient() *http.Client {
	return &http.Client{Transport: NewMigrationHTTPTransport()}
}

// getMigrationHTTPClient returns the shared migration client.
func getMigrationHTTPClient() *http.Client {
	return migrationHTTPClient()
}

// NewMigrationHTTPTransport returns a HTTP transport for migration. The shared policy validates the
// target on both the direct-dial and proxy paths, so a configured proxy cannot be used to reach an
// otherwise-forbidden target (SSRF).
func NewMigrationHTTPTransport() *http.Transport {
	t := egress.GetMigrationPolicy().NewHTTPTransport()
	t.TLSClientConfig = &tls.Config{InsecureSkipVerify: setting.Migrations.SkipTLSVerify}
	return t
}
