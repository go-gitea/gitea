// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSSHFrom_Defaults(t *testing.T) {
	defer test.MockVariableValue(&SSH)()

	cfg, err := NewConfigProviderFromData(`
[server]
DOMAIN = example.com
`)
	require.NoError(t, err)
	loadSSHFrom(cfg)

	assert.NotContains(t, SSH.ServerKeyExchanges, "diffie-hellman-group14-sha1")
	assert.NotContains(t, SSH.ServerKeyExchanges, "diffie-hellman-group1-sha1")
	assert.NotContains(t, SSH.ServerKeyExchanges, "ecdh-sha2-nistp256")
	assert.NotContains(t, SSH.ServerKeyExchanges, "ecdh-sha2-nistp384")
	assert.NotContains(t, SSH.ServerKeyExchanges, "ecdh-sha2-nistp521")

	assert.NotContains(t, SSH.ServerMACs, "hmac-sha1")
	assert.NotContains(t, SSH.ServerMACs, "hmac-sha1-96")

	assert.Contains(t, SSH.ServerKeyExchanges, "curve25519-sha256")
	assert.Contains(t, SSH.ServerMACs, "hmac-sha2-256-etm@openssh.com")
	assert.Contains(t, SSH.ServerCiphers, "chacha20-poly1305@openssh.com")

	assert.True(t, strings.HasSuffix(SSH.ServerHostKeys[0], "ssh/gitea.ed25519"), "unexpected first host key: %s", SSH.ServerHostKeys[0])
}

func TestLoadSSHFrom_CustomAlgorithms(t *testing.T) {
	defer test.MockVariableValue(&SSH)()

	cfg, err := NewConfigProviderFromData(`
[server]
DOMAIN = example.com
SSH_SERVER_CIPHERS = aes256-ctr
SSH_SERVER_KEY_EXCHANGES = diffie-hellman-group14-sha256
SSH_SERVER_MACS = hmac-sha2-256
`)
	require.NoError(t, err)
	loadSSHFrom(cfg)

	// User-supplied values must override the secure defaults.
	assert.Equal(t, []string{"aes256-ctr"}, SSH.ServerCiphers)
	assert.Equal(t, []string{"diffie-hellman-group14-sha256"}, SSH.ServerKeyExchanges)
	assert.Equal(t, []string{"hmac-sha2-256"}, SSH.ServerMACs)
}

func TestLoadSSHFrom_EmptyAlgorithmsUsesDefaults(t *testing.T) {
	defer test.MockVariableValue(&SSH)()

	cfg, err := NewConfigProviderFromData(`
[server]
DOMAIN = example.com
SSH_SERVER_CIPHERS =
SSH_SERVER_KEY_EXCHANGES =
SSH_SERVER_MACS =
`)
	require.NoError(t, err)
	loadSSHFrom(cfg)

	assert.Equal(t, defaultServerCiphers, SSH.ServerCiphers)
	assert.Equal(t, defaultServerKeyExchanges, SSH.ServerKeyExchanges)
	assert.Equal(t, defaultServerMACs, SSH.ServerMACs)
}
