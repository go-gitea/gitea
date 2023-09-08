// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db_test

import (
	"os"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestBackupRestore(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	d, err := os.MkdirTemp(os.TempDir(), "backup_restore")
	assert.NoError(t, err)

	assert.NoError(t, db.BackupDatabaseAsFixtures(d))

	f, err := os.Open(d)
	assert.NoError(t, err)
	defer f.Close()

	entries, err := f.ReadDir(0)
	assert.NoError(t, err)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileEqual(t, filepath.Join("..", "fixtures", entry.Name()), filepath.Join(d, entry.Name()))
	}

	// assert.NoError(t, db.RestoreDatabase(d))
}

func fileEqual(t *testing.T, a, b string) {
	bs1, err := os.ReadFile(a)
	assert.NoError(t, err)

	bs2, err := os.ReadFile(b)
	assert.NoError(t, err)
	assert.EqualValues(t, bs1, bs2)
}
