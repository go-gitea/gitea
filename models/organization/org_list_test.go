// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestCountOrganizations(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	expected, err := db.GetEngine(db.DefaultContext).Where("type=?", user_model.UserTypeOrganization).Count(&organization.Organization{})
	assert.NoError(t, err)
	cnt, err := db.Count[organization.Organization](db.DefaultContext, organization.FindOrgOptions{IncludePrivate: true})
	assert.NoError(t, err)
	assert.Equal(t, expected, cnt)
}

func TestFindOrgs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	orgs, err := db.Find[organization.Organization](db.DefaultContext, organization.FindOrgOptions{
		UserID:         4,
		IncludePrivate: true,
	})
	assert.NoError(t, err)
	if assert.Len(t, orgs, 1) {
		assert.EqualValues(t, 3, orgs[0].ID)
	}

	orgs, err = db.Find[organization.Organization](db.DefaultContext, organization.FindOrgOptions{
		UserID:         4,
		IncludePrivate: false,
	})
	assert.NoError(t, err)
	assert.Empty(t, orgs)

	total, err := db.Count[organization.Organization](db.DefaultContext, organization.FindOrgOptions{
		UserID:         4,
		IncludePrivate: true,
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, total)
}

func TestGetUserOrgsList(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	orgs, err := organization.GetUserOrgsList(db.DefaultContext, &user_model.User{ID: 4})
	assert.NoError(t, err)
	if assert.Len(t, orgs, 1) {
		assert.EqualValues(t, 3, orgs[0].ID)
		// repo_id: 3 is in the team, 32 is public, 5 is private with no team
		assert.EqualValues(t, 2, orgs[0].NumRepos)
	}
}

func TestLoadOrgListTeams(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	orgs, err := organization.GetUserOrgsList(db.DefaultContext, &user_model.User{ID: 4})
	assert.NoError(t, err)
	assert.Len(t, orgs, 1)
	teamsMap, err := organization.OrgList(orgs).LoadTeams(db.DefaultContext)
	assert.NoError(t, err)
	assert.Len(t, teamsMap, 1)
	assert.Len(t, teamsMap[3], 5)
}
