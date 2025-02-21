// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestUser_RemoveMember(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	// remove a user that is a member
	unittest.AssertExistsAndLoadBean(t, &organization.OrgUser{UID: user4.ID, OrgID: org.ID})
	prevNumMembers := org.NumMembers
	assert.NoError(t, RemoveOrgUser(db.DefaultContext, org, user4))
	unittest.AssertNotExistsBean(t, &organization.OrgUser{UID: user4.ID, OrgID: org.ID})

	org = unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: org.ID})
	assert.Equal(t, prevNumMembers-1, org.NumMembers)

	// remove a user that is not a member
	unittest.AssertNotExistsBean(t, &organization.OrgUser{UID: user5.ID, OrgID: org.ID})
	prevNumMembers = org.NumMembers
	assert.NoError(t, RemoveOrgUser(db.DefaultContext, org, user5))
	unittest.AssertNotExistsBean(t, &organization.OrgUser{UID: user5.ID, OrgID: org.ID})

	org = unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: org.ID})
	assert.Equal(t, prevNumMembers, org.NumMembers)

	unittest.CheckConsistencyFor(t, &user_model.User{}, &organization.Team{})
}

func TestRemoveOrgUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(org *organization.Organization, user *user_model.User) {
		expectedNumMembers := org.NumMembers
		if unittest.GetBean(t, &organization.OrgUser{OrgID: org.ID, UID: user.ID}) != nil {
			expectedNumMembers--
		}
		assert.NoError(t, RemoveOrgUser(db.DefaultContext, org, user))
		unittest.AssertNotExistsBean(t, &organization.OrgUser{OrgID: org.ID, UID: user.ID})
		org = unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: org.ID})
		assert.EqualValues(t, expectedNumMembers, org.NumMembers)
	}

	org3 := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	org7 := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 7})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	testSuccess(org3, user4)

	org3 = unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	testSuccess(org3, user4)

	err := RemoveOrgUser(db.DefaultContext, org7, user5)
	assert.Error(t, err)
	assert.True(t, organization.IsErrLastOrgOwner(err))
	unittest.AssertExistsAndLoadBean(t, &organization.OrgUser{OrgID: org7.ID, UID: user5.ID})
	unittest.CheckConsistencyFor(t, &user_model.User{}, &organization.Team{})
}
