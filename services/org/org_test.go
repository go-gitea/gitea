// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package org

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, filepath.Join("..", ".."))
}

func TestDeleteOrganization(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	org := unittest.AssertExistsAndLoadBean(t, &models.Organization{ID: 6}).(*models.Organization)
	assert.NoError(t, DeleteOrganization(org))
	unittest.AssertNotExistsBean(t, &models.Organization{ID: 6})
	unittest.AssertNotExistsBean(t, &models.OrgUser{OrgID: 6})
	unittest.AssertNotExistsBean(t, &models.Team{OrgID: 6})

	org = unittest.AssertExistsAndLoadBean(t, &models.Organization{ID: 3}).(*models.Organization)
	err := DeleteOrganization(org)
	assert.Error(t, err)
	assert.True(t, models.IsErrUserOwnRepos(err))

	user := unittest.AssertExistsAndLoadBean(t, &models.Organization{ID: 5}).(*models.Organization)
	assert.Error(t, DeleteOrganization(user))
	unittest.CheckConsistencyFor(t, &user_model.User{}, &models.Team{})
}
