// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestUserListIsPublicMember(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	tt := []struct {
		orgid    int64
		expected []bool
	}{
		{3, []bool{true, false}},
		{6, []bool{true}},
		{7, []bool{false}},
	}
	for _, v := range tt {
		t.Run(fmt.Sprintf("IsPublicMemberOfOrdIg%d", v.orgid), func(t *testing.T) {
			testUserListIsPublicMember(t, v.orgid, v.expected)
		})
	}
}
func testUserListIsPublicMember(t *testing.T, orgID int64, expected []bool) {
	org, err := GetUserByID(orgID)
	assert.NoError(t, err)
	assert.NoError(t, org.GetMembers())
	assert.Equal(t, expected, org.Members.IsPublicMember(orgID))

}

func TestUserListIsUserOrgOwner(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	tt := []struct {
		orgid    int64
		expected []bool
	}{
		{3, []bool{true, false}},
		{6, []bool{true}},
		{7, []bool{false}},
	}
	for _, v := range tt {
		t.Run(fmt.Sprintf("IsUserOrgOwnerOfOrdIg%d", v.orgid), func(t *testing.T) {
			testUserListIsUserOrgOwner(t, v.orgid, v.expected)
		})
	}
}
func testUserListIsUserOrgOwner(t *testing.T, orgID int64, expected []bool) {
	org, err := GetUserByID(orgID)
	assert.NoError(t, err)
	assert.NoError(t, org.GetMembers())
	assert.Equal(t, expected, org.Members.IsPublicMember(orgID))

}

func TestUserListIsTwoFaEnrolled(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	tt := []struct {
		orgid    int64
		expected []bool
	}{
		{3, []bool{true, false}},
		{6, []bool{false}},
		{7, []bool{false}},
	}
	for _, v := range tt {
		t.Run(fmt.Sprintf("IsTwoFaEnrolledOfOrdIg%d", v.orgid), func(t *testing.T) {
			testUserListIsTwoFaEnrolled(t, v.orgid, v.expected)
		})
	}
}
func testUserListIsTwoFaEnrolled(t *testing.T, orgID int64, expected []bool) {
	org, err := GetUserByID(orgID)
	assert.NoError(t, err)
	assert.NoError(t, org.GetMembers())
	assert.Equal(t, expected, org.Members.IsTwoFaEnrolled())

}
