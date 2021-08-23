// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"io/ioutil"
	"os"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/migrations"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestMigrateLocalPath(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())

	adminUser := models.AssertExistsAndLoadBean(t, &models.User{Name: "user1"}).(*models.User)

	old := setting.ImportLocalPaths
	setting.ImportLocalPaths = true

	lowercasePath, err := ioutil.TempDir("", "lowercase") // may not be lowercase because TempDir creates a random directory name which may be mixedcase
	assert.NoError(t, err)
	defer os.RemoveAll(lowercasePath)

	err = migrations.IsMigrateURLAllowed(lowercasePath, adminUser)
	assert.NoError(t, err, "case lowercase path")

	mixedcasePath, err := ioutil.TempDir("", "mIxeDCaSe")
	assert.NoError(t, err)
	defer os.RemoveAll(mixedcasePath)

	err = migrations.IsMigrateURLAllowed(mixedcasePath, adminUser)
	assert.NoError(t, err, "case mixedcase path")

	setting.ImportLocalPaths = old
}
