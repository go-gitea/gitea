// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadDBSettingSQLitePath(t *testing.T) {
	oldDB := Database
	oldWorkPath := AppWorkPath
	oldDataPath := AppDataPath
	t.Cleanup(func() {
		Database = oldDB
		AppWorkPath = oldWorkPath
		AppDataPath = oldDataPath
	})

	AppWorkPath = "/srv/gitea"
	AppDataPath = "/srv/gitea/data"

	t.Run("relative path uses work path", func(t *testing.T) {
		cfg, err := NewConfigProviderFromData("[database]\nDB_TYPE=sqlite3\nPATH=data/custom.db\n")
		assert.NoError(t, err)

		loadDBSetting(cfg)
		assert.Equal(t, filepath.Clean("/srv/gitea/data/custom.db"), filepath.Clean(Database.Path))
	})

	t.Run("absolute path stays absolute", func(t *testing.T) {
		cfg, err := NewConfigProviderFromData("[database]\nDB_TYPE=sqlite3\nPATH=/var/lib/gitea/gitea.db\n")
		assert.NoError(t, err)

		loadDBSetting(cfg)
		assert.Equal(t, "/var/lib/gitea/gitea.db", filepath.Clean(Database.Path))
	})

	t.Run("memory path stays in memory", func(t *testing.T) {
		cfg, err := NewConfigProviderFromData("[database]\nDB_TYPE=sqlite3\nPATH=:memory:\n")
		assert.NoError(t, err)

		loadDBSetting(cfg)
		assert.Equal(t, ":memory:", Database.Path)
	})

	t.Run("sqlite uri stays unchanged", func(t *testing.T) {
		cfg, err := NewConfigProviderFromData("[database]\nDB_TYPE=sqlite3\nPATH=file:test.db?cache=shared\n")
		assert.NoError(t, err)

		loadDBSetting(cfg)
		assert.Equal(t, "file:test.db?cache=shared", Database.Path)
	})
}
