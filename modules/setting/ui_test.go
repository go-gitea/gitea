// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFolderIconThemeDefault(t *testing.T) {
	t.Run("DefaultsToBasic", func(t *testing.T) {
		cfg, err := NewConfigProviderFromData(`
[ui]
FILE_ICON_THEME = material
`)
		assert.NoError(t, err)

		loadUIFrom(cfg)

		assert.Equal(t, "material", UI.FileIconTheme)
		assert.Equal(t, "basic", UI.FolderIconTheme, "FolderIconTheme should default to basic")
	})

	t.Run("UsesExplicitValue", func(t *testing.T) {
		cfg, err := NewConfigProviderFromData(`
[ui]
FILE_ICON_THEME = material
FOLDER_ICON_THEME = material
`)
		assert.NoError(t, err)

		loadUIFrom(cfg)

		assert.Equal(t, "material", UI.FileIconTheme)
		assert.Equal(t, "material", UI.FolderIconTheme, "FolderIconTheme should use explicit value")
	})

	t.Run("BothBasic", func(t *testing.T) {
		cfg, err := NewConfigProviderFromData(`
[ui]
FILE_ICON_THEME = basic
FOLDER_ICON_THEME = basic
`)
		assert.NoError(t, err)

		loadUIFrom(cfg)

		assert.Equal(t, "basic", UI.FileIconTheme)
		assert.Equal(t, "basic", UI.FolderIconTheme)
	})
}
