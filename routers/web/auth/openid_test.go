// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"testing"

	"code.gitea.io/gitea/modules/hostmatcher"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestAllowedOpenIDURI(t *testing.T) {
	defer test.MockVariableValue(&setting.Service)()

	t.Run("Whitelist", func(t *testing.T) {
		setting.OpenID.Allowlist = hostmatcher.ParseHostMatchList("openid.WHITELISTED_URIS", "trusted.example.com,*.trusted.net")
		setting.OpenID.Blocklist = hostmatcher.ParseHostMatchList("openid.BLACKLISTED_URIS", "trusted.example.com,bad.example.com")

		assert.NoError(t, allowedOpenIDURI("https://trusted.example.com/openid"))
		assert.NoError(t, allowedOpenIDURI("https://sub.trusted.net"))
		assert.Error(t, allowedOpenIDURI("https://trusted.example.com.evil.org"))
		assert.Error(t, allowedOpenIDURI("https://bad.example.com"))
	})

	t.Run("Blacklist", func(t *testing.T) {
		setting.OpenID.Allowlist = nil
		setting.OpenID.Blocklist = hostmatcher.ParseHostMatchList("openid.BLACKLISTED_URIS", "bad.example.com,10.0.0.0/8")

		assert.Error(t, allowedOpenIDURI("https://bad.example.com"))
		assert.Error(t, allowedOpenIDURI("https://10.1.1.1"))
		assert.NoError(t, allowedOpenIDURI("https://good.example.com"))
	})

	t.Run("InvalidURI", func(t *testing.T) {
		setting.OpenID.Allowlist = nil
		setting.OpenID.Blocklist = nil

		assert.Error(t, allowedOpenIDURI("://bad"))
	})
}
