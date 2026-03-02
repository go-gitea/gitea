// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"crypto/tls"
	"net"
	"net/http"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/hostmatcher"
	"code.gitea.io/gitea/modules/proxy"
	"code.gitea.io/gitea/modules/setting"
)

// NewMigrationHTTPClient returns a HTTP client for migration
func NewMigrationHTTPClient() *http.Client {
	return &http.Client{
		Transport:     NewMigrationHTTPTransport(),
		CheckRedirect: CheckMigrateRedirect,
	}
}

func CheckMigrateRedirect(req *http.Request, via []*http.Request) error {
	redirectURL := req.URL
	if redirectURL == nil {
		return &git.ErrInvalidCloneAddr{IsURLError: true, Host: "<EMPTY_REDIRECT_URL>"}
	}
	if redirectURL.Scheme != "http" && redirectURL.Scheme != "https" {
		return &git.ErrInvalidCloneAddr{Host: redirectURL.Host, IsProtocolInvalid: true, IsPermissionDenied: true, IsURLError: true}
	}
	hostName := redirectURL.Hostname()
	if hostName == "" {
		return &git.ErrInvalidCloneAddr{IsURLError: true, Host: redirectURL.String()}
	}
	addrList, _ := net.LookupIP(hostName)
	return checkByAllowBlockList(hostName, addrList)
}

// NewMigrationHTTPTransport returns a HTTP transport for migration
func NewMigrationHTTPTransport() *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: setting.Migrations.SkipTLSVerify},
		Proxy:           proxy.Proxy(),
		DialContext:     hostmatcher.NewDialContext("migration", allowList, blockList, setting.Proxy.ProxyURLFixed),
	}
}
