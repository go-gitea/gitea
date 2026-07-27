// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"path/filepath"
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
		assert.Nil(t, Audit.FileOptions)
	})

	t.Run("FileSinkDisabled", func(t *testing.T) {
		defer test.MockVariableValue(&Audit)()

		cfg, err := NewConfigProviderFromData(`
[audit]
ENABLED = true

[audit.file]
ENABLED = false
`)
		require.NoError(t, err)
		loadAuditFrom(cfg)

		assert.True(t, Audit.Enabled)
		assert.Nil(t, Audit.FileOptions)
	})

	t.Run("RelativeFileName", func(t *testing.T) {
		defer test.MockVariableValue(&Audit)()
		defer test.MockVariableValue(&Log.RootPath, t.TempDir())()

		cfg, err := NewConfigProviderFromData(`
[audit]
ENABLED = true

[audit.file]
ENABLED = true
FILE_NAME = sub/audit.log
MAX_DAYS = 3
COMPRESS = false
`)
		require.NoError(t, err)
		loadAuditFrom(cfg)

		require.NotNil(t, Audit.FileOptions)
		// a relative FILE_NAME must resolve below the log root instead of failing
		assert.Equal(t, filepath.Join(Log.RootPath, "sub", "audit.log"), Audit.FileOptions.FileName)
		assert.Equal(t, 3, Audit.FileOptions.MaxDays)
		assert.False(t, Audit.FileOptions.Compress)
		assert.Equal(t, int64(1<<28), Audit.FileOptions.MaxSize)
	})

	t.Run("AbsoluteFileNameAndDefaults", func(t *testing.T) {
		defer test.MockVariableValue(&Audit)()
		defer test.MockVariableValue(&Log.RootPath, t.TempDir())()

		absName := filepath.Join(t.TempDir(), "custom-audit.log")
		cfg, err := NewConfigProviderFromData(`
[audit]
ENABLED = true

[audit.file]
ENABLED = true
FILE_NAME = ` + absName + `
MAXIMUM_SIZE = 1mb
`)
		require.NoError(t, err)
		loadAuditFrom(cfg)

		require.NotNil(t, Audit.FileOptions)
		assert.Equal(t, absName, Audit.FileOptions.FileName)
		assert.Equal(t, int64(1000000), Audit.FileOptions.MaxSize)
		assert.True(t, Audit.FileOptions.LogRotate)
		assert.True(t, Audit.FileOptions.DailyRotate)
		assert.Equal(t, 7, Audit.FileOptions.MaxDays)
	})
}
