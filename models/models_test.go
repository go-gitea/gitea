// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestDumpDatabase(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	dir, err := ioutil.TempDir(os.TempDir(), "dump")
	assert.NoError(t, err)

	for _, dbName := range setting.SupportedDatabases {
		dbType := setting.GetDBTypeByName(dbName)
		assert.NoError(t, DumpDatabase(filepath.Join(dir, dbType+".sql"), dbType))
	}
}
