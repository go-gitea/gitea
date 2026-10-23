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

// getMigrationHTTPClient returns the shared migration client, so downloads from one host reuse its connections
var getMigrationHTTPClient = sync.OnceValue(newMigrationHTTPClient)

// newMigrationHTTPClient returns a HTTP client for migration
func newMigrationHTTPClient() *http.Client {
	return &http.Client{Transport: NewMigrationHTTPTransport()}
}

// NewMigrationHTTPTransport returns a HTTP transport for migration, enforcing the migration policy on its direct dials.
func NewMigrationHTTPTransport() *http.Transport {
	t := egress.NewMigrationPolicy().NewHTTPTransport()
	t.TLSClientConfig = &tls.Config{InsecureSkipVerify: setting.Migrations.SkipTLSVerify}
	return t
}
