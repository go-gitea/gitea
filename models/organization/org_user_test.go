// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization_test

import (
	"fmt"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestUserIsPublicMember(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	tt := []struct {
		uid      int64
		orgid    int64
		expected bool
	}{
		{2, 3, true},
		{4, 3, false},
		{5, 6, true},
		{5, 7, false},
	}
	for _, v := range tt {
		t.Run(fmt.Sprintf("UserId%dIsPublicMemberOf%d", v.uid, v.orgid), func(t *testing.T) {
			testUserIsPublicMember(t, v.uid, v.orgid, v.expected)
		})
	}
}

func testUserIsPublicMember(t *testing.T, uid, orgID int64, expected bool) {
	user, err := user_model.GetUserByID(db.DefaultContext, uid)
	assert.NoError(t, err)
	is, err := organization.IsPublicMembership(db.DefaultContext, orgID, user.ID)
	assert.NoError(t, err)
	assert.Equal(t, expected, is)
}

func TestIsUserOrgOwner(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	tt := []struct {
		uid      int64
		orgid    int64
		expected bool
	}{
		{2, 3, true},
		{4, 3, false},
		{5, 6, true},
		{5, 7, true},
	}
	for _, v := range tt {
		t.Run(fmt.Sprintf("UserId%dIsOrgOwnerOf%d", v.uid, v.orgid), func(t *testing.T) {
			testIsUserOrgOwner(t, v.uid, v.orgid, v.expected)
		})
	}
}

func testIsUserOrgOwner(t *testing.T, uid, orgID int64, expected bool) {
	user, err := user_model.GetUserByID(db.DefaultContext, uid)
	assert.NoError(t, err)
	is, err := organization.IsOrganizationOwner(db.DefaultContext, orgID, user.ID)
	assert.NoError(t, err)
	assert.Equal(t, expected, is)
}

func TestUserListIsPublicMember(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	tt := []struct {
		orgid    int64
		expected map[int64]bool
	}{
		{3, map[int64]bool{2: true, 4: false, 28: true}},
		{6, map[int64]bool{5: true, 28: true}},
		{7, map[int64]bool{5: false}},
		{25, map[int64]bool{12: true, 24: true}},
		{22, map[int64]bool{}},
	}
	for _, v := range tt {
		t.Run(fmt.Sprintf("IsPublicMemberOfOrgId%d", v.orgid), func(t *testing.T) {
			testUserListIsPublicMember(t, v.orgid, v.expected)
		})
	}
}

func testUserListIsPublicMember(t *testing.T, orgID int64, expected map[int64]bool) {
	org, err := organization.GetOrgByID(db.DefaultContext, orgID)
	assert.NoError(t, err)
	_, membersIsPublic, err := org.GetMembers(db.DefaultContext, &user_model.User{IsAdmin: true})
	assert.NoError(t, err)
	assert.Equal(t, expected, membersIsPublic)
}

func TestUserListIsUserOrgOwner(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	tt := []struct {
		orgid    int64
		expected map[int64]bool
	}{
		{3, map[int64]bool{2: true, 4: false, 28: false}},
		{6, map[int64]bool{5: true, 28: false}},
		{7, map[int64]bool{5: true}},
		{25, map[int64]bool{12: true, 24: false}}, // ErrTeamNotExist
		{22, map[int64]bool{}},                    // No member
	}
	for _, v := range tt {
		t.Run(fmt.Sprintf("IsUserOrgOwnerOfOrgId%d", v.orgid), func(t *testing.T) {
			testUserListIsUserOrgOwner(t, v.orgid, v.expected)
		})
	}
}

func testUserListIsUserOrgOwner(t *testing.T, orgID int64, expected map[int64]bool) {
	org, err := organization.GetOrgByID(db.DefaultContext, orgID)
	assert.NoError(t, err)
	members, _, err := org.GetMembers(db.DefaultContext, &user_model.User{IsAdmin: true})
	assert.NoError(t, err)
	assert.Equal(t, expected, organization.IsUserOrgOwner(db.DefaultContext, members, orgID))
}

func TestAddOrgUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	testSuccess := func(orgID, userID int64, isPublic bool) {
		org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: orgID})
		expectedNumMembers := org.NumMembers
		if unittest.GetBean(t, &organization.OrgUser{OrgID: orgID, UID: userID}) == nil {
			expectedNumMembers++
		}
		assert.NoError(t, organization.AddOrgUser(db.DefaultContext, orgID, userID))
		ou := &organization.OrgUser{OrgID: orgID, UID: userID}
		unittest.AssertExistsAndLoadBean(t, ou)
		assert.Equal(t, isPublic, ou.IsPublic)
		org = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: orgID})
		assert.EqualValues(t, expectedNumMembers, org.NumMembers)
	}

	setting.Service.DefaultOrgMemberVisible = false
	testSuccess(3, 5, false)
	testSuccess(3, 5, false)
	testSuccess(6, 2, false)

	setting.Service.DefaultOrgMemberVisible = true
	testSuccess(6, 3, true)

	unittest.CheckConsistencyFor(t, &user_model.User{}, &organization.Team{})
}
