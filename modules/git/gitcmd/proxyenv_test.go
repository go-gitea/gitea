// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitcmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoveProxyEnvsIfConfigured(t *testing.T) {
	env := []string{"PATH=/usr/bin", "no_proxy=*", "NO_PROXY=10.0.0.0/8", "https_proxy=http://corp:3128", "HOME=/home/git"}

	t.Run("KeptWithoutProxyConfig", func(t *testing.T) {
		assert.Equal(t, env, removeProxyEnvsIfConfigured(env, []string{"-c", "http.sslVerify=false"}))
	})

	t.Run("StrippedWithProxyConfig", func(t *testing.T) {
		// git has no http.noProxy key, so an inherited no_proxy would otherwise make libcurl ignore
		// the configured proxy and defeat the redirect guard
		got := removeProxyEnvsIfConfigured(env, []string{"-c", "http.proxy=http://127.0.0.1:1234"})
		assert.Equal(t, []string{"PATH=/usr/bin", "HOME=/home/git"}, got)
	})
}
