// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCodespaceFrom(t *testing.T) {
	cfg, err := NewConfigProviderFromData(`
[codespace]
ENABLED = false
GIT_PROTOCOL = ssh
GIT_SSH_KNOWN_HOSTS = gitea.example.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIH6Y4idVaW3E+bLw1uqoAfJD7o5Siu+HqS51E9oQLPE9,[gitea.example.com]:2222 ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCtest
GATEWAY_REQUIRE_HTTPS = true
CONTROL_PLANE_TIMEOUT = 10s
MANAGER_OFFLINE_TIMEOUT = 80s
OPERATION_LEASE_TIMEOUT = 1500ms
OPERATION_MAX_DURATION = 3h
QUEUE_TIMEOUT = 7m
LOG_MAX_SIZE = 32MiB
AUTO_STOP_DEFAULT_TIMEOUT = 25m
AUTO_STOP_MIN_TIMEOUT = 3m
AUTO_STOP_MAX_TIMEOUT = 24h
`)
	require.NoError(t, err)
	loadCodespaceFrom(cfg)
	assert.False(t, Codespace.Enabled)
	assert.Equal(t, "ssh", Codespace.GitProtocol)
	assert.Len(t, Codespace.GitSSHKnownHosts, 2)
	assert.True(t, Codespace.GatewayRequireHTTPS)
	assert.Equal(t, 10*time.Second, Codespace.ControlPlaneTimeout)
	assert.Equal(t, 80*time.Second, Codespace.ManagerOfflineTimeout)
	assert.Equal(t, 1500*time.Millisecond, Codespace.OperationLeaseTimeout)
	assert.Equal(t, 3*time.Hour, Codespace.OperationMaxDuration)
	assert.Equal(t, 7*time.Minute, Codespace.QueueTimeout)
	assert.EqualValues(t, 32*1024*1024, Codespace.LogMaxSize)
	assert.Equal(t, 25*time.Minute, Codespace.AutoStopDefaultTimeout)
	assert.Equal(t, 3*time.Minute, Codespace.AutoStopMinTimeout)
	assert.Equal(t, 24*time.Hour, Codespace.AutoStopMaxTimeout)

	cfg, err = NewConfigProviderFromData(`
[codespace]
GIT_PROTOCOL = invalid
`)
	require.NoError(t, err)
	loadCodespaceFrom(cfg)
	assert.True(t, Codespace.Enabled)
	assert.Equal(t, "invalid", Codespace.GitProtocol)
	assert.False(t, Codespace.GatewayRequireHTTPS)
	assert.Equal(t, 30*time.Second, Codespace.ControlPlaneTimeout)
}
