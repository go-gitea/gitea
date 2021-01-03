// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserListIsPublicMember(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	tt := []struct {
		orgid    int64
		expected map[int64]bool
	}{
		{3, map[int64]bool{2: true, 4: false, 28: true}},
		{6, map[int64]bool{5: true, 28: true}},
		{7, map[int64]bool{5: false}},
		{25, map[int64]bool{24: true}},
		{22, map[int64]bool{}},
	}
	for _, v := range tt {
		t.Run(fmt.Sprintf("IsPublicMemberOfOrdIg%d", v.orgid), func(t *testing.T) {
			testUserListIsPublicMember(t, v.orgid, v.expected)
		})
	}
}
func testUserListIsPublicMember(t *testing.T, orgID int64, expected map[int64]bool) {
	org, err := GetUserByID(orgID)
	assert.NoError(t, err)
	assert.NoError(t, org.GetMembers())
	assert.Equal(t, expected, org.MembersIsPublic)

}

func TestUserListIsUserOrgOwner(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	tt := []struct {
		orgid    int64
		expected map[int64]bool
	}{
		{3, map[int64]bool{2: true, 4: false, 28: false}},
		{6, map[int64]bool{5: true, 28: false}},
		{7, map[int64]bool{5: true}},
		{25, map[int64]bool{24: false}}, // ErrTeamNotExist
		{22, map[int64]bool{}},          // No member
	}
	for _, v := range tt {
		t.Run(fmt.Sprintf("IsUserOrgOwnerOfOrdIg%d", v.orgid), func(t *testing.T) {
			testUserListIsUserOrgOwner(t, v.orgid, v.expected)
		})
	}
}

func testUserListIsUserOrgOwner(t *testing.T, orgID int64, expected map[int64]bool) {
	org, err := GetUserByID(orgID)
	assert.NoError(t, err)
	assert.NoError(t, org.GetMembers())
	assert.Equal(t, expected, org.Members.IsUserOrgOwner(orgID))
}

func TestUserListIsTwoFaEnrolled(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	tt := []struct {
		orgid    int64
		expected map[int64]bool
	}{
		{3, map[int64]bool{2: false, 4: false, 28: false}},
		{6, map[int64]bool{5: false, 28: false}},
		{7, map[int64]bool{5: false}},
		{25, map[int64]bool{24: true}},
		{22, map[int64]bool{}},
	}
	for _, v := range tt {
		t.Run(fmt.Sprintf("IsTwoFaEnrolledOfOrdIg%d", v.orgid), func(t *testing.T) {
			testUserListIsTwoFaEnrolled(t, v.orgid, v.expected)
		})
	}
}

func testUserListIsTwoFaEnrolled(t *testing.T, orgID int64, expected map[int64]bool) {
	org, err := GetUserByID(orgID)
	assert.NoError(t, err)
	assert.NoError(t, org.GetMembers())
	assert.Equal(t, expected, org.Members.GetTwoFaStatus())
}
