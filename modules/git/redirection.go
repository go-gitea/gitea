// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"net/url"
	"strings"
	"sync/atomic"

	"gitea.dev/modules/git/gitcmd"
)

// httpProxyProvider yields the address of a loopback proxy that validates every HTTP target,
// redirect hops included, against the migration allow/block lists. It is registered by the
// migrations service, which owns those lists; modules/git cannot import services/, hence the hook.
var httpProxyProvider atomic.Pointer[func() string]

// SetGitCmdHTTPProxyProvider registers the provider used by HandleGitCmdHTTPRedirection. Passing
// nil clears it, restoring the fallback behaviour.
func SetGitCmdHTTPProxyProvider(provider func() string) {
	if provider == nil {
		httpProxyProvider.Store(nil)
		return
	}
	httpProxyProvider.Store(&provider)
}

// HandleGitCmdHTTPRedirection protects a git subprocess from being redirected onto an address the
// allow/block lists forbid (SSRF, e.g. migrating from an attacker-controlled URL).
//
// Gitea validates a migration URL by resolving it once and checking the result, but `git` then
// resolves DNS again and follows HTTP redirects on its own, so an approved external host can answer
// 30x and send the subprocess to an internal address. Routing the subprocess through the validating
// proxy puts every hop back under the same policy: a redirect to a permitted target still resolves,
// a redirect to a forbidden one is refused at connect time.
//
// Blanket `http.followRedirects=false` was tried before and reverted: it also rejects the benign
// redirects real forges emit, e.g. GitLab answering 301 for a clone URL without the .git suffix.
// It stays only as the fail-closed fallback for the rare caller with no proxy available.
func HandleGitCmdHTTPRedirection(cmd *gitcmd.Command, targets ...string) {
	if provider := httpProxyProvider.Load(); provider != nil {
		if proxyURL := (*provider)(); proxyURL != "" {
			cmd.AddConfig("http.proxy", proxyURL)
			return
		}
	}
	cmd.AddConfig("http.followRedirects", "false")
}

// isHTTPRemote reports whether a clone source is an HTTP(S) URL, i.e. a remote the redirect guard
// applies to. Local paths and other transports are left alone.
func isHTTPRemote(from string) bool {
	u, err := url.Parse(from)
	return err == nil && (strings.EqualFold(u.Scheme, "http") || strings.EqualFold(u.Scheme, "https"))
}
