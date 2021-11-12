// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestMigrateWhiteBlocklist(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	adminUser := db.AssertExistsAndLoadBean(t, &models.User{Name: "user1"}).(*models.User)
	nonAdminUser := db.AssertExistsAndLoadBean(t, &models.User{Name: "user2"}).(*models.User)

	setting.Migrations.AllowedDomains = []string{"github.com"}
	assert.NoError(t, Init())

	err := IsMigrateURLAllowed("https://gitlab.com/gitlab/gitlab.git", nonAdminUser)
	assert.Error(t, err)

	err = IsMigrateURLAllowed("https://github.com/go-gitea/gitea.git", nonAdminUser)
	assert.NoError(t, err)

	err = IsMigrateURLAllowed("https://gITHUb.com/go-gitea/gitea.git", nonAdminUser)
	assert.NoError(t, err)

	setting.Migrations.AllowedDomains = []string{}
	setting.Migrations.BlockedDomains = []string{"github.com"}
	assert.NoError(t, Init())

	err = IsMigrateURLAllowed("https://gitlab.com/gitlab/gitlab.git", nonAdminUser)
	assert.NoError(t, err)

	err = IsMigrateURLAllowed("https://github.com/go-gitea/gitea.git", nonAdminUser)
	assert.Error(t, err)

	err = IsMigrateURLAllowed("https://10.0.0.1/go-gitea/gitea.git", nonAdminUser)
	assert.Error(t, err)

	setting.Migrations.AllowLocalNetworks = true
	err = IsMigrateURLAllowed("https://10.0.0.1/go-gitea/gitea.git", nonAdminUser)
	assert.NoError(t, err)

	old := setting.ImportLocalPaths
	setting.ImportLocalPaths = false

	err = IsMigrateURLAllowed("/home/foo/bar/goo", adminUser)
	assert.Error(t, err)

	setting.ImportLocalPaths = true
	abs, err := filepath.Abs(".")
	assert.NoError(t, err)

	err = IsMigrateURLAllowed(abs, adminUser)
	assert.NoError(t, err)

	err = IsMigrateURLAllowed(abs, nonAdminUser)
	assert.Error(t, err)

	nonAdminUser.AllowImportLocal = true
	err = IsMigrateURLAllowed(abs, nonAdminUser)
	assert.NoError(t, err)

	setting.ImportLocalPaths = old
}
