// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAuditFrom(t *testing.T) {
	t.Run("DisabledByDefault", func(t *testing.T) {
		defer test.MockVariableValue(&Audit)()

		cfg, err := NewConfigProviderFromData("")
		require.NoError(t, err)
		loadAuditFrom(cfg)

		assert.False(t, Audit.Enabled)
	})

	t.Run("Enabled", func(t *testing.T) {
		defer test.MockVariableValue(&Audit)()

		cfg, err := NewConfigProviderFromData(`
[audit]
ENABLED = true
`)
		require.NoError(t, err)
		loadAuditFrom(cfg)

		assert.True(t, Audit.Enabled)
	})
}
