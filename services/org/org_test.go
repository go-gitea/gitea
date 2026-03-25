// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"testing"

	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestOrg(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	t.Run("UpdateOrgEmailAddress", func(t *testing.T) {
		org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
		originalEmail := org.Email

		require.NoError(t, UpdateOrgEmailAddress(t.Context(), org, nil))
		unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3, Email: originalEmail})

		newEmail := "contact@org3.example.com"
		require.NoError(t, UpdateOrgEmailAddress(t.Context(), org, &newEmail))
		unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3, Email: newEmail})

		invalidEmail := "invalid email"
		err := UpdateOrgEmailAddress(t.Context(), org, &invalidEmail)
		require.ErrorIs(t, err, util.ErrInvalidArgument)
		unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3, Email: newEmail})

		require.NoError(t, UpdateOrgEmailAddress(t.Context(), org, new("")))
		org = unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3, Email: ""})
		assert.Empty(t, org.Email)
	})

	t.Run("DeleteOrganization", func(t *testing.T) {
		org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 6})
		assert.NoError(t, DeleteOrganization(t.Context(), org, false))
		unittest.AssertNotExistsBean(t, &organization.Organization{ID: 6})
		unittest.AssertNotExistsBean(t, &organization.OrgUser{OrgID: 6})
		unittest.AssertNotExistsBean(t, &organization.Team{OrgID: 6})

		org = unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
		err := DeleteOrganization(t.Context(), org, false)
		assert.Error(t, err)
		assert.True(t, repo_model.IsErrUserOwnRepos(err))

		user := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 5})
		assert.Error(t, DeleteOrganization(t.Context(), user, false))
		unittest.CheckConsistencyFor(t, &user_model.User{}, &organization.Team{})
	})
}
