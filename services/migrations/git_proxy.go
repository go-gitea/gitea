// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"crypto/tls"
	"sync"

	"gitea.dev/modules/git"
	"gitea.dev/modules/hostmatcher"
	"gitea.dev/modules/log"
	"gitea.dev/modules/proxy"
	"gitea.dev/modules/setting"
)

// The git subprocesses used for migrations and mirror syncs cannot be given an http.Transport, so
// they get the same SSRF policy through a loopback proxy instead: see
// git.HandleGitCmdHTTPRedirection. Registration happens at package load rather than in Init so that
// no caller can reach a git clone with the hook unset; the proxy itself is started on first use and
// rebuilt whenever Init reparses the lists. Until Init has run there are no lists to enforce, so the
// proxy refuses to start and the caller falls back to refusing redirects outright.
func init() {
	git.SetGitCmdHTTPProxyProvider(gitHTTPProxyURL)
}

var (
	gitProxyMu     sync.Mutex
	gitProxyServer *hostmatcher.ProxyServer
	gitProxyFailed bool
)

// gitHTTPProxyURL returns the address of the validating proxy, starting it on first use. It returns
// an empty string if the proxy cannot be started, which makes the caller fail closed.
func gitHTTPProxyURL() string {
	gitProxyMu.Lock()
	defer gitProxyMu.Unlock()

	if gitProxyServer != nil {
		return gitProxyServer.URL()
	}
	if gitProxyFailed {
		return ""
	}

	server, err := hostmatcher.NewProxyServer("migration", allowList, blockList, proxy.Proxy(), setting.Proxy.ProxyURLFixed,
		&tls.Config{InsecureSkipVerify: setting.Migrations.SkipTLSVerify})
	if err != nil {
		// remember the failure so every migration does not retry a listen that is not going to work
		gitProxyFailed = true
		log.Error("Unable to start the migration git proxy, redirects will be refused instead: %v", err)
		return ""
	}
	gitProxyServer = server
	return server.URL()
}

// resetGitHTTPProxy drops the running proxy so the next use rebuilds it from the current
// allow/block lists.
func resetGitHTTPProxy() {
	gitProxyMu.Lock()
	defer gitProxyMu.Unlock()

	if gitProxyServer != nil {
		if err := gitProxyServer.Close(); err != nil {
			log.Error("Unable to stop the previous migration git proxy: %v", err)
		}
		gitProxyServer = nil
	}
	gitProxyFailed = false
}
