// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"gitea.dev/models/organization"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestAdminRemoveUserFromOrg(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	adminSession := loginUser(t, "user1")

	t.Run("RemoveFromOrg", func(t *testing.T) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
		org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})

		isMember, err := organization.IsOrganizationMember(t.Context(), org.ID, user.ID)
		assert.NoError(t, err)
		assert.True(t, isMember)

		req := NewRequest(t, "POST", "/-/admin/users/4/orgs/3/remove")
		adminSession.MakeRequest(t, req, http.StatusOK)

		isMember, err = organization.IsOrganizationMember(t.Context(), org.ID, user.ID)
		assert.NoError(t, err)
		assert.False(t, isMember)
	})

	t.Run("RemoveFromAllOrg", func(t *testing.T) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

		orgCount, err := organization.GetOrganizationCount(t.Context(), user)
		assert.EqualValues(t, 4, orgCount)
		assert.NoError(t, err)
		assert.Positive(t, orgCount, "User should be in at least one org")

		req := NewRequest(t, "POST", "/-/admin/users/5/orgs/remove-all")
		adminSession.MakeRequest(t, req, http.StatusOK)

		orgCountAfter, err := organization.GetOrganizationCount(t.Context(), user)
		assert.NoError(t, err)
		assert.EqualValues(t, 2, orgCountAfter) // User 5 is the last owner of remaining orgs
	})
}
