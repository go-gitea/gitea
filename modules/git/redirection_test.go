// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"strings"
	"testing"

	"gitea.dev/modules/git/gitcmd"

	"github.com/stretchr/testify/assert"
)

func TestHandleGitCmdHTTPRedirection(t *testing.T) {
	t.Run("RoutesThroughValidatingProxy", func(t *testing.T) {
		SetGitCmdHTTPProxyProvider(func() string { return "http://127.0.0.1:65000" })
		t.Cleanup(func() { SetGitCmdHTTPProxyProvider(nil) })

		cmd := gitcmd.NewCommand("clone")
		HandleGitCmdHTTPRedirection(cmd)
		assert.Contains(t, strings.Join(cmd.ConfigArgs(), " "), "http.proxy=http://127.0.0.1:65000")
		assert.NotContains(t, strings.Join(cmd.ConfigArgs(), " "), "http.followRedirects")
	})

	t.Run("FailsClosedWithoutProvider", func(t *testing.T) {
		SetGitCmdHTTPProxyProvider(nil)

		cmd := gitcmd.NewCommand("clone")
		HandleGitCmdHTTPRedirection(cmd)
		assert.Contains(t, strings.Join(cmd.ConfigArgs(), " "), "http.followRedirects=false")
	})

	t.Run("FailsClosedWhenProxyUnavailable", func(t *testing.T) {
		SetGitCmdHTTPProxyProvider(func() string { return "" })
		t.Cleanup(func() { SetGitCmdHTTPProxyProvider(nil) })

		cmd := gitcmd.NewCommand("clone")
		HandleGitCmdHTTPRedirection(cmd)
		assert.Contains(t, strings.Join(cmd.ConfigArgs(), " "), "http.followRedirects=false")
	})
}

func TestIsHTTPRemote(t *testing.T) {
	for source, want := range map[string]bool{
		"https://example.com/o/r.git": true,
		"http://example.com/o/r.git":  true,
		"HTTPS://example.com/o/r.git": true,
		"git://example.com/o/r.git":   false,
		"ssh://git@example.com/o/r":   false,
		"git@example.com:o/r.git":     false,
		"/var/lib/gitea/repos/o/r":    false,
		"":                            false,
	} {
		assert.Equal(t, want, isHTTPRemote(source), source)
	}
}
