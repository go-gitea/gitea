// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package egress

var gitProxyURL string

// SetGitProxyURL records the running internal git proxy's base URL, e.g.
// "http://127.0.0.1:37891". Called once at startup, before any git command runs; an empty
// value clears it (used by tests).
func SetGitProxyURL(baseURL string) { gitProxyURL = baseURL }

// GitProxyURL returns the internal git proxy base URL, or "" if it is not running.
func GitProxyURL() string { return gitProxyURL }
