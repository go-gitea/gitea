// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package org

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, filepath.Join("..", ".."))
}

func TestDeleteOrganization(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := unittest.AssertExistsAndLoadBean(t, &models.User{ID: 6}).(*models.User)
	assert.NoError(t, DeleteOrganization(org))
	unittest.AssertNotExistsBean(t, &models.User{ID: 6})
	unittest.AssertNotExistsBean(t, &models.OrgUser{OrgID: 6})
	unittest.AssertNotExistsBean(t, &models.Team{OrgID: 6})

	org = unittest.AssertExistsAndLoadBean(t, &models.User{ID: 3}).(*models.User)
	err := DeleteOrganization(org)
	assert.Error(t, err)
	assert.True(t, models.IsErrUserOwnRepos(err))

	user := unittest.AssertExistsAndLoadBean(t, &models.User{ID: 5}).(*models.User)
	assert.Error(t, DeleteOrganization(user))
	unittest.CheckConsistencyFor(t, &models.User{}, &models.Team{})
}
