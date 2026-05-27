// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"crypto/tls"
	"net/http"

	"gitea.dev/modules/hostmatcher"
	"gitea.dev/modules/proxy"
	"gitea.dev/modules/setting"
)

// NewMigrationHTTPClient returns a HTTP client for migration
func NewMigrationHTTPClient() *http.Client {
	return &http.Client{
		Transport: NewMigrationHTTPTransport(),
	}
}

// NewMigrationHTTPTransport returns a HTTP transport for migration
func NewMigrationHTTPTransport() *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: setting.Migrations.SkipTLSVerify},
		Proxy:           proxy.Proxy(),
		DialContext:     hostmatcher.NewDialContext("migration", allowList, blockList, setting.Proxy.ProxyURLFixed),
	}
}
