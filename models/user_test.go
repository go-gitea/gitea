// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestGetUserEmailsByNames(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	// ignore none active user email
	assert.Equal(t, []string{"user8@example.com"}, GetUserEmailsByNames([]string{"user8", "user9"}))
	assert.Equal(t, []string{"user8@example.com", "user5@example.com"}, GetUserEmailsByNames([]string{"user8", "user5"}))
}

func TestCanCreateOrganization(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	admin := AssertExistsAndLoadBean(t, &User{ID: 1}).(*User)
	assert.True(t, admin.CanCreateOrganization())

	user := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	assert.True(t, user.CanCreateOrganization())
	// Disable user create organization permission.
	user.AllowCreateOrganization = false
	assert.False(t, user.CanCreateOrganization())

	setting.Admin.DisableRegularOrgCreation = true
	user.AllowCreateOrganization = true
	assert.True(t, admin.CanCreateOrganization())
	assert.False(t, user.CanCreateOrganization())
}

func TestDeleteUser(t *testing.T) {
	test := func(userID int64) {
		assert.NoError(t, PrepareTestDatabase())
		user := AssertExistsAndLoadBean(t, &User{ID: userID}).(*User)

		ownedRepos := make([]*Repository, 0, 10)
		assert.NoError(t, x.Find(&ownedRepos, &Repository{OwnerID: userID}))
		if len(ownedRepos) > 0 {
			err := DeleteUser(user)
			assert.Error(t, err)
			assert.True(t, IsErrUserOwnRepos(err))
			return
		}

		orgUsers := make([]*OrgUser, 0, 10)
		assert.NoError(t, x.Find(&orgUsers, &OrgUser{UID: userID}))
		for _, orgUser := range orgUsers {
			if err := RemoveOrgUser(orgUser.OrgID, orgUser.UID); err != nil {
				assert.True(t, IsErrLastOrgOwner(err))
				return
			}
		}
		assert.NoError(t, DeleteUser(user))
		AssertNotExistsBean(t, &User{ID: userID})
		CheckConsistencyFor(t, &User{}, &Repository{})
	}
	test(2)
	test(4)
	test(8)
	test(11)
}
