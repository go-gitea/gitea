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

	// Admin user
	session := loginUser(t, "user1")

	// User to remove from org
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})

	// Verify user is in org
	isMember, err := organization.IsOrganizationMember(t.Context(), org.ID, user.ID)
	assert.NoError(t, err)
	assert.True(t, isMember)

	// Remove user from org
	req := NewRequest(t, "POST", "/-/admin/users/4/orgs/3/remove")
	session.MakeRequest(t, req, http.StatusSeeOther)

	// Verify user is no longer in org
	isMember, err = organization.IsOrganizationMember(t.Context(), org.ID, user.ID)
	assert.NoError(t, err)
	assert.False(t, isMember)
}

func TestAdminRemoveUserFromAllOrgs(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// Admin user
	session := loginUser(t, "user1")

	// User to remove from all orgs (user4 is not a last owner)
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	// Get count of orgs user is in before removal
	orgCount, err := organization.GetOrganizationCount(t.Context(), user)
	assert.NoError(t, err)
	assert.Positive(t, orgCount, "User should be in at least one org")

	// Remove user from all orgs
	req := NewRequest(t, "POST", "/-/admin/users/4/orgs/remove-all")
	session.MakeRequest(t, req, http.StatusSeeOther)

	// Verify user is no longer in any orgs
	orgCountAfter, err := organization.GetOrganizationCount(t.Context(), user)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), orgCountAfter, "User should not be in any orgs")
}
