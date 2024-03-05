// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestDeleteOrganization(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 6})
	assert.NoError(t, DeleteOrganization(db.DefaultContext, user, org, false))
	unittest.AssertNotExistsBean(t, &organization.Organization{ID: 6})
	unittest.AssertNotExistsBean(t, &organization.OrgUser{OrgID: 6})
	unittest.AssertNotExistsBean(t, &organization.Team{OrgID: 6})

	org = unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	err := DeleteOrganization(db.DefaultContext, user, org, false)
	assert.Error(t, err)
	assert.True(t, models.IsErrUserOwnRepos(err))

	assert.Error(t, DeleteOrganization(db.DefaultContext, user, organization.OrgFromUser(user), false))
	unittest.CheckConsistencyFor(t, &user_model.User{}, &organization.Team{})
}
