// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestUserIsPublicMember(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

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

func testUserIsPublicMember(t *testing.T, uid int64, orgID int64, expected bool) {
	user, err := GetUserByID(uid)
	assert.NoError(t, err)
	assert.Equal(t, expected, user.IsPublicMember(orgID))
}

func TestIsUserOrgOwner(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

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

func testIsUserOrgOwner(t *testing.T, uid int64, orgID int64, expected bool) {
	user, err := GetUserByID(uid)
	assert.NoError(t, err)
	assert.Equal(t, expected, user.IsUserOrgOwner(orgID))
}

func TestGetUserEmailsByNames(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	// ignore none active user email
	assert.Equal(t, []string{"user8@example.com"}, GetUserEmailsByNames([]string{"user8", "user9"}))
	assert.Equal(t, []string{"user8@example.com", "user5@example.com"}, GetUserEmailsByNames([]string{"user8", "user5"}))

	assert.Equal(t, []string{"user8@example.com"}, GetUserEmailsByNames([]string{"user8", "user7"}))
}

func TestUser_APIFormat(t *testing.T) {

	user, err := GetUserByID(1)
	assert.NoError(t, err)
	assert.True(t, user.IsAdmin)

	apiUser := user.APIFormat()
	assert.True(t, apiUser.IsAdmin)

	user, err = GetUserByID(2)
	assert.NoError(t, err)
	assert.False(t, user.IsAdmin)

	apiUser = user.APIFormat()
	assert.False(t, apiUser.IsAdmin)
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

func TestSearchUsers(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	testSuccess := func(opts *SearchUserOptions, expectedUserOrOrgIDs []int64) {
		users, _, err := SearchUsers(opts)
		assert.NoError(t, err)
		if assert.Len(t, users, len(expectedUserOrOrgIDs)) {
			for i, expectedID := range expectedUserOrOrgIDs {
				assert.EqualValues(t, expectedID, users[i].ID)
			}
		}
	}

	// test orgs
	testOrgSuccess := func(opts *SearchUserOptions, expectedOrgIDs []int64) {
		opts.Type = UserTypeOrganization
		testSuccess(opts, expectedOrgIDs)
	}

	testOrgSuccess(&SearchUserOptions{OrderBy: "id ASC", Page: 1, PageSize: 2},
		[]int64{3, 6})

	testOrgSuccess(&SearchUserOptions{OrderBy: "id ASC", Page: 2, PageSize: 2},
		[]int64{7, 17})

	testOrgSuccess(&SearchUserOptions{OrderBy: "id ASC", Page: 3, PageSize: 2},
		[]int64{19, 25})

	testOrgSuccess(&SearchUserOptions{OrderBy: "id ASC", Page: 4, PageSize: 2},
		[]int64{26})

	testOrgSuccess(&SearchUserOptions{Page: 5, PageSize: 2},
		[]int64{})

	// test users
	testUserSuccess := func(opts *SearchUserOptions, expectedUserIDs []int64) {
		opts.Type = UserTypeIndividual
		testSuccess(opts, expectedUserIDs)
	}

	testUserSuccess(&SearchUserOptions{OrderBy: "id ASC", Page: 1},
		[]int64{1, 2, 4, 5, 8, 9, 10, 11, 12, 13, 14, 15, 16, 18, 20, 21, 24, 27, 28})

	testUserSuccess(&SearchUserOptions{Page: 1, IsActive: util.OptionalBoolFalse},
		[]int64{9})

	testUserSuccess(&SearchUserOptions{OrderBy: "id ASC", Page: 1, IsActive: util.OptionalBoolTrue},
		[]int64{1, 2, 4, 5, 8, 10, 11, 12, 13, 14, 15, 16, 18, 20, 21, 24, 28})

	testUserSuccess(&SearchUserOptions{Keyword: "user1", OrderBy: "id ASC", Page: 1, IsActive: util.OptionalBoolTrue},
		[]int64{1, 10, 11, 12, 13, 14, 15, 16, 18})

	// order by name asc default
	testUserSuccess(&SearchUserOptions{Keyword: "user1", Page: 1, IsActive: util.OptionalBoolTrue},
		[]int64{1, 10, 11, 12, 13, 14, 15, 16, 18})
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

func TestEmailNotificationPreferences(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	for _, test := range []struct {
		expected string
		userID   int64
	}{
		{EmailNotificationsEnabled, 1},
		{EmailNotificationsEnabled, 2},
		{EmailNotificationsOnMention, 3},
		{EmailNotificationsOnMention, 4},
		{EmailNotificationsEnabled, 5},
		{EmailNotificationsEnabled, 6},
		{EmailNotificationsDisabled, 7},
		{EmailNotificationsEnabled, 8},
		{EmailNotificationsOnMention, 9},
	} {
		user := AssertExistsAndLoadBean(t, &User{ID: test.userID}).(*User)
		assert.Equal(t, test.expected, user.EmailNotifications())

		// Try all possible settings
		assert.NoError(t, user.SetEmailNotifications(EmailNotificationsEnabled))
		assert.Equal(t, EmailNotificationsEnabled, user.EmailNotifications())

		assert.NoError(t, user.SetEmailNotifications(EmailNotificationsOnMention))
		assert.Equal(t, EmailNotificationsOnMention, user.EmailNotifications())

		assert.NoError(t, user.SetEmailNotifications(EmailNotificationsDisabled))
		assert.Equal(t, EmailNotificationsDisabled, user.EmailNotifications())
	}
}

func TestHashPasswordDeterministic(t *testing.T) {
	b := make([]byte, 16)
	rand.Read(b)
	u := &User{Salt: string(b)}
	algos := []string{"pbkdf2", "argon2", "scrypt", "bcrypt"}
	for j := 0; j < len(algos); j++ {
		u.PasswdHashAlgo = algos[j]
		for i := 0; i < 50; i++ {
			// generate a random password
			rand.Read(b)
			pass := string(b)

			// save the current password in the user - hash it and store the result
			u.HashPassword(pass)
			r1 := u.Passwd

			// run again
			u.HashPassword(pass)
			r2 := u.Passwd

			// assert equal (given the same salt+pass, the same result is produced) except bcrypt
			if u.PasswdHashAlgo == "bcrypt" {
				assert.NotEqual(t, r1, r2)
			} else {
				assert.Equal(t, r1, r2)
			}
		}
	}
}

func BenchmarkHashPassword(b *testing.B) {
	// BenchmarkHashPassword ensures that it takes a reasonable amount of time
	// to hash a password - in order to protect from brute-force attacks.
	pass := "password1337"
	bs := make([]byte, 16)
	rand.Read(bs)
	u := &User{Salt: string(bs), Passwd: pass}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		u.HashPassword(pass)
	}
}

func TestGetOrgRepositoryIDs(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	user2 := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	user4 := AssertExistsAndLoadBean(t, &User{ID: 4}).(*User)
	user5 := AssertExistsAndLoadBean(t, &User{ID: 5}).(*User)

	accessibleRepos, err := user2.GetOrgRepositoryIDs()
	assert.NoError(t, err)
	// User 2's team has access to private repos 3, 5, repo 32 is a public repo of the organization
	assert.Equal(t, []int64{3, 5, 23, 24, 32}, accessibleRepos)

	accessibleRepos, err = user4.GetOrgRepositoryIDs()
	assert.NoError(t, err)
	// User 4's team has access to private repo 3, repo 32 is a public repo of the organization
	assert.Equal(t, []int64{3, 32}, accessibleRepos)

	accessibleRepos, err = user5.GetOrgRepositoryIDs()
	assert.NoError(t, err)
	// User 5's team has no access to any repo
	assert.Len(t, accessibleRepos, 0)
}

func TestNewGitSig(t *testing.T) {
	users := make([]*User, 0, 20)
	sess := x.NewSession()
	defer sess.Close()
	sess.Find(&users)

	for _, user := range users {
		sig := user.NewGitSig()
		assert.NotContains(t, sig.Name, "<")
		assert.NotContains(t, sig.Name, ">")
		assert.NotContains(t, sig.Name, "\n")
		assert.NotEqual(t, len(strings.TrimSpace(sig.Name)), 0)
	}
}

func TestDisplayName(t *testing.T) {
	users := make([]*User, 0, 20)
	sess := x.NewSession()
	defer sess.Close()
	sess.Find(&users)

	for _, user := range users {
		displayName := user.DisplayName()
		assert.Equal(t, strings.TrimSpace(displayName), displayName)
		if len(strings.TrimSpace(user.FullName)) == 0 {
			assert.Equal(t, user.Name, displayName)
		}
		assert.NotEqual(t, len(strings.TrimSpace(displayName)), 0)
	}
}

func TestCreateUser(t *testing.T) {
	user := &User{
		Name:               "GiteaBot",
		Email:              "GiteaBot@gitea.io",
		Passwd:             ";p['////..-++']",
		IsAdmin:            false,
		Theme:              setting.UI.DefaultTheme,
		MustChangePassword: false,
	}

	assert.NoError(t, CreateUser(user))

	assert.NoError(t, DeleteUser(user))
}

func TestCreateUser_Issue5882(t *testing.T) {

	// Init settings
	_ = setting.Admin

	passwd := ".//.;1;;//.,-=_"

	tt := []struct {
		user               *User
		disableOrgCreation bool
	}{
		{&User{Name: "GiteaBot", Email: "GiteaBot@gitea.io", Passwd: passwd, MustChangePassword: false}, false},
		{&User{Name: "GiteaBot2", Email: "GiteaBot2@gitea.io", Passwd: passwd, MustChangePassword: false}, true},
	}

	setting.Service.DefaultAllowCreateOrganization = true

	for _, v := range tt {
		setting.Admin.DisableRegularOrgCreation = v.disableOrgCreation

		assert.NoError(t, CreateUser(v.user))

		u, err := GetUserByEmail(v.user.Email)
		assert.NoError(t, err)

		assert.Equal(t, !u.AllowCreateOrganization, v.disableOrgCreation)

		assert.NoError(t, DeleteUser(v.user))
	}
}

func TestGetUserIDsByNames(t *testing.T) {

	//ignore non existing
	IDs, err := GetUserIDsByNames([]string{"user1", "user2", "none_existing_user"}, true)
	assert.NoError(t, err)
	assert.Equal(t, []int64{1, 2}, IDs)

	//ignore non existing
	IDs, err = GetUserIDsByNames([]string{"user1", "do_not_exist"}, false)
	assert.Error(t, err)
	assert.Equal(t, []int64(nil), IDs)
}
