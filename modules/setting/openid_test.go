// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestLoadOpenIDSettings(t *testing.T) {
	defer test.MockVariableValue(&Service)()

	t.Run("DefaultBlacklist", func(t *testing.T) {
		cfg, err := NewConfigProviderFromData(`
[openid]
`)
		assert.NoError(t, err)
		loadServiceFrom(cfg)

		assert.False(t, OpenID.EnableSignIn)
		assert.False(t, OpenID.EnableSignUp)
		assert.Nil(t, OpenID.Allowlist)
		if assert.NotNil(t, OpenID.Blocklist) {
			assert.True(t, OpenID.Blocklist.MatchHostName("localhost"))
			assert.True(t, OpenID.Blocklist.MatchHostName("127.0.0.1"))
			assert.True(t, OpenID.Blocklist.MatchHostName("192.168.0.1"))
			assert.True(t, OpenID.Blocklist.MatchHostName("fe80::1"))
			assert.False(t, OpenID.Blocklist.MatchHostName("example.com"))
		}
	})

	t.Run("AllowlistParsing", func(t *testing.T) {
		cfg, err := NewConfigProviderFromData(`
[openid]
ENABLE_OPENID_SIGNIN = true
WHITELISTED_URIS = example.com, *.trusted.example
`)
		assert.NoError(t, err)
		loadServiceFrom(cfg)

		assert.True(t, OpenID.EnableSignIn)
		assert.True(t, OpenID.EnableSignUp)
		if assert.NotNil(t, OpenID.Allowlist) {
			assert.True(t, OpenID.Allowlist.MatchHostName("example.com"))
			assert.True(t, OpenID.Allowlist.MatchHostName("foo.trusted.example"))
			assert.False(t, OpenID.Allowlist.MatchHostName("example.org"))
		}
		if assert.NotNil(t, OpenID.Blocklist) {
			assert.True(t, OpenID.Blocklist.MatchHostName("localhost"))
		}
	})

	t.Run("BlocklistParsing", func(t *testing.T) {
		cfg, err := NewConfigProviderFromData(`
[openid]
BLACKLISTED_URIS = bad.example.com, 10.0.0.0/8
`)
		assert.NoError(t, err)
		loadServiceFrom(cfg)

		assert.Nil(t, OpenID.Allowlist)
		if assert.NotNil(t, OpenID.Blocklist) {
			assert.True(t, OpenID.Blocklist.MatchHostName("bad.example.com"))
			assert.True(t, OpenID.Blocklist.MatchHostName("10.1.1.1"))
			assert.False(t, OpenID.Blocklist.MatchHostName("localhost"))
			assert.False(t, OpenID.Blocklist.MatchHostName("good.example.com"))
		}
	})
}
