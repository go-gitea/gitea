// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestLoadUIEnableEmojiDropdown(t *testing.T) {
	t.Run("default enabled", func(t *testing.T) {
		defer test.MockVariableValue(&UI)()

		cfg, err := NewConfigProviderFromData(`
[ui]
`)
		assert.NoError(t, err)
		loadUIFrom(cfg)
		assert.True(t, UI.EnableEmojiDropdown)
	})

	t.Run("can be disabled", func(t *testing.T) {
		defer test.MockVariableValue(&UI)()

		cfg, err := NewConfigProviderFromData(`
[ui]
ENABLE_EMOJI_DROPDOWN = false
`)
		assert.NoError(t, err)
		loadUIFrom(cfg)
		assert.False(t, UI.EnableEmojiDropdown)
	})
}
