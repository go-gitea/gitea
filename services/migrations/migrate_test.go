// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestMigrateWhiteBlocklist(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	adminUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user1"}).(*user_model.User)
	nonAdminUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2"}).(*user_model.User)

	setting.Migrations.AllowedDomains = "github.com"
	setting.Migrations.AllowLocalNetworks = false
	assert.NoError(t, Init())

	err := IsMigrateURLAllowed("https://gitlab.com/gitlab/gitlab.git", nonAdminUser)
	assert.Error(t, err)

	err = IsMigrateURLAllowed("https://github.com/go-gitea/gitea.git", nonAdminUser)
	assert.NoError(t, err)

	err = IsMigrateURLAllowed("https://gITHUb.com/go-gitea/gitea.git", nonAdminUser)
	assert.NoError(t, err)

	setting.Migrations.AllowedDomains = ""
	setting.Migrations.BlockedDomains = "github.com"
	assert.NoError(t, Init())

	err = IsMigrateURLAllowed("https://gitlab.com/gitlab/gitlab.git", nonAdminUser)
	assert.NoError(t, err)

	err = IsMigrateURLAllowed("https://github.com/go-gitea/gitea.git", nonAdminUser)
	assert.Error(t, err)

	err = IsMigrateURLAllowed("https://10.0.0.1/go-gitea/gitea.git", nonAdminUser)
	assert.Error(t, err)

	setting.Migrations.AllowLocalNetworks = true
	assert.NoError(t, Init())
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
