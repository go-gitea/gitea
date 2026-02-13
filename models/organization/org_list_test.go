// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestOrgList(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	t.Run("CountOrganizations", testCountOrganizations)
	t.Run("FindOrgs", testFindOrgs)
	t.Run("GetUserOrgsList", testGetUserOrgsList)
	t.Run("LoadOrgListTeams", testLoadOrgListTeams)
	t.Run("DoerViewOtherVisibility", testDoerViewOtherVisibility)
}

func testCountOrganizations(t *testing.T) {
	expected, err := db.GetEngine(t.Context()).Where("type=?", user_model.UserTypeOrganization).Count(&organization.Organization{})
	assert.NoError(t, err)
	cnt, err := db.Count[organization.Organization](t.Context(), organization.FindOrgOptions{IncludeVisibility: structs.VisibleTypePrivate})
	assert.NoError(t, err)
	assert.Equal(t, expected, cnt)
}

func testFindOrgs(t *testing.T) {
	orgs, err := db.Find[organization.Organization](t.Context(), organization.FindOrgOptions{
		UserID:            4,
		IncludeVisibility: structs.VisibleTypePrivate,
	})
	assert.NoError(t, err)
	if assert.Len(t, orgs, 1) {
		assert.EqualValues(t, 3, orgs[0].ID)
	}

	orgs, err = db.Find[organization.Organization](t.Context(), organization.FindOrgOptions{
		UserID: 4,
	})
	assert.NoError(t, err)
	assert.Empty(t, orgs)

	total, err := db.Count[organization.Organization](t.Context(), organization.FindOrgOptions{
		UserID:            4,
		IncludeVisibility: structs.VisibleTypePrivate,
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, total)
}

func testGetUserOrgsList(t *testing.T) {
	orgs, err := organization.GetUserOrgsList(t.Context(), &user_model.User{ID: 4})
	assert.NoError(t, err)
	if assert.Len(t, orgs, 1) {
		assert.EqualValues(t, 3, orgs[0].ID)
		// repo_id: 3 is in the team, 32 is public, 5 is private with no team
		assert.Equal(t, 2, orgs[0].NumRepos)
	}
}

func testLoadOrgListTeams(t *testing.T) {
	orgs, err := organization.GetUserOrgsList(t.Context(), &user_model.User{ID: 4})
	assert.NoError(t, err)
	assert.Len(t, orgs, 1)
	teamsMap, err := organization.OrgList(orgs).LoadTeams(t.Context())
	assert.NoError(t, err)
	assert.Len(t, teamsMap, 1)
	assert.Len(t, teamsMap[3], 5)
}

func testDoerViewOtherVisibility(t *testing.T) {
	assert.Equal(t, structs.VisibleTypePublic, organization.DoerViewOtherVisibility(nil, nil))
	assert.Equal(t, structs.VisibleTypeLimited, organization.DoerViewOtherVisibility(&user_model.User{ID: 1}, &user_model.User{ID: 2}))
	assert.Equal(t, structs.VisibleTypePrivate, organization.DoerViewOtherVisibility(&user_model.User{ID: 1}, &user_model.User{ID: 1}))
	assert.Equal(t, structs.VisibleTypePrivate, organization.DoerViewOtherVisibility(&user_model.User{ID: 1, IsAdmin: true}, &user_model.User{ID: 2}))
}
