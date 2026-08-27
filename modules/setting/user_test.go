// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestLoadUserKeyLimits(t *testing.T) {
	defer test.MockVariableValue(&User.MaxSSHKeysPerUser)()
	defer test.MockVariableValue(&User.MaxGPGKeysPerUser)()

	t.Run("Defaults", func(t *testing.T) {
		cfg, err := NewConfigProviderFromData("")
		assert.NoError(t, err)
		loadUserFrom(cfg)
		assert.Equal(t, 8, User.MaxSSHKeysPerUser)
		assert.Equal(t, 8, User.MaxGPGKeysPerUser)
	})

	t.Run("Overrides", func(t *testing.T) {
		cfg, err := NewConfigProviderFromData(`
[user]
MAX_SSH_KEYS_PER_USER = 20
MAX_GPG_KEYS_PER_USER = -1
`)
		assert.NoError(t, err)
		loadUserFrom(cfg)
		assert.Equal(t, 20, User.MaxSSHKeysPerUser)
		assert.Equal(t, -1, User.MaxGPGKeysPerUser)
	})
}
