// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"
	"time"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSSHPerWriteTimeoutMinusOne(t *testing.T) {
	defer test.MockVariableValue(&SSH)()

	SSH.ServerHostKeys = nil
	SSH.MinimumKeySizes = map[string]int{}

	cfg, err := NewConfigProviderFromData(`
[server]
SSH_PER_WRITE_TIMEOUT = -1
SSH_PER_WRITE_PER_KB_TIMEOUT = -1
`)
	require.NoError(t, err)

	loadSSHFrom(cfg)

	assert.Equal(t, -time.Nanosecond, SSH.PerWriteTimeout)
	assert.Equal(t, -time.Nanosecond, SSH.PerWritePerKbTimeout)
}
