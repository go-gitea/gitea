// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"math/rand"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/login"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestOAuth2Application_LoadUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	app := unittest.AssertExistsAndLoadBean(t, &login.OAuth2Application{ID: 1}).(*login.OAuth2Application)
	user, err := GetUserByID(app.UID)
	assert.NoError(t, err)
	assert.NotNil(t, user)
}

func TestGetUserEmailsByNames(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// ignore none active user email
	assert.Equal(t, []string{"user8@example.com"}, GetUserEmailsByNames([]string{"user8", "user9"}))
	assert.Equal(t, []string{"user8@example.com", "user5@example.com"}, GetUserEmailsByNames([]string{"user8", "user5"}))

	assert.Equal(t, []string{"user8@example.com"}, GetUserEmailsByNames([]string{"user8", "user7"}))
}

func TestCanCreateOrganization(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	admin := unittest.AssertExistsAndLoadBean(t, &User{ID: 1}).(*User)
	assert.True(t, admin.CanCreateOrganization())

	user := unittest.AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
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
	assert.NoError(t, unittest.PrepareTestDatabase())
	testSuccess := func(opts *SearchUserOptions, expectedUserOrOrgIDs []int64) {
		users, _, err := SearchUsers(opts)
		assert.NoError(t, err)
		if assert.Len(t, users, len(expectedUserOrOrgIDs), opts) {
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

	testOrgSuccess(&SearchUserOptions{OrderBy: "id ASC", ListOptions: db.ListOptions{Page: 1, PageSize: 2}},
		[]int64{3, 6})

	testOrgSuccess(&SearchUserOptions{OrderBy: "id ASC", ListOptions: db.ListOptions{Page: 2, PageSize: 2}},
		[]int64{7, 17})

	testOrgSuccess(&SearchUserOptions{OrderBy: "id ASC", ListOptions: db.ListOptions{Page: 3, PageSize: 2}},
		[]int64{19, 25})

	testOrgSuccess(&SearchUserOptions{OrderBy: "id ASC", ListOptions: db.ListOptions{Page: 4, PageSize: 2}},
		[]int64{26})

	testOrgSuccess(&SearchUserOptions{ListOptions: db.ListOptions{Page: 5, PageSize: 2}},
		[]int64{})

	// test users
	testUserSuccess := func(opts *SearchUserOptions, expectedUserIDs []int64) {
		opts.Type = UserTypeIndividual
		testSuccess(opts, expectedUserIDs)
	}

	testUserSuccess(&SearchUserOptions{OrderBy: "id ASC", ListOptions: db.ListOptions{Page: 1}},
		[]int64{1, 2, 4, 5, 8, 9, 10, 11, 12, 13, 14, 15, 16, 18, 20, 21, 24, 27, 28, 29, 30, 32})

	testUserSuccess(&SearchUserOptions{ListOptions: db.ListOptions{Page: 1}, IsActive: util.OptionalBoolFalse},
		[]int64{9})

	testUserSuccess(&SearchUserOptions{OrderBy: "id ASC", ListOptions: db.ListOptions{Page: 1}, IsActive: util.OptionalBoolTrue},
		[]int64{1, 2, 4, 5, 8, 10, 11, 12, 13, 14, 15, 16, 18, 20, 21, 24, 28, 29, 30, 32})

	testUserSuccess(&SearchUserOptions{Keyword: "user1", OrderBy: "id ASC", ListOptions: db.ListOptions{Page: 1}, IsActive: util.OptionalBoolTrue},
		[]int64{1, 10, 11, 12, 13, 14, 15, 16, 18})

	// order by name asc default
	testUserSuccess(&SearchUserOptions{Keyword: "user1", ListOptions: db.ListOptions{Page: 1}, IsActive: util.OptionalBoolTrue},
		[]int64{1, 10, 11, 12, 13, 14, 15, 16, 18})

	testUserSuccess(&SearchUserOptions{ListOptions: db.ListOptions{Page: 1}, IsAdmin: util.OptionalBoolTrue},
		[]int64{1})

	testUserSuccess(&SearchUserOptions{ListOptions: db.ListOptions{Page: 1}, IsRestricted: util.OptionalBoolTrue},
		[]int64{29, 30})

	testUserSuccess(&SearchUserOptions{ListOptions: db.ListOptions{Page: 1}, IsProhibitLogin: util.OptionalBoolTrue},
		[]int64{30})

	testUserSuccess(&SearchUserOptions{ListOptions: db.ListOptions{Page: 1}, IsTwoFactorEnabled: util.OptionalBoolTrue},
		[]int64{24})
}

func TestEmailNotificationPreferences(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

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
		user := unittest.AssertExistsAndLoadBean(t, &User{ID: test.userID}).(*User)
		assert.Equal(t, test.expected, user.EmailNotifications())

		// Try all possible settings
		assert.NoError(t, SetEmailNotifications(user, EmailNotificationsEnabled))
		assert.Equal(t, EmailNotificationsEnabled, user.EmailNotifications())

		assert.NoError(t, SetEmailNotifications(user, EmailNotificationsOnMention))
		assert.Equal(t, EmailNotificationsOnMention, user.EmailNotifications())

		assert.NoError(t, SetEmailNotifications(user, EmailNotificationsDisabled))
		assert.Equal(t, EmailNotificationsDisabled, user.EmailNotifications())
	}
}

func TestHashPasswordDeterministic(t *testing.T) {
	b := make([]byte, 16)
	u := &User{}
	algos := []string{"argon2", "pbkdf2", "scrypt", "bcrypt"}
	for j := 0; j < len(algos); j++ {
		u.PasswdHashAlgo = algos[j]
		for i := 0; i < 50; i++ {
			// generate a random password
			rand.Read(b)
			pass := string(b)

			// save the current password in the user - hash it and store the result
			u.SetPassword(pass)
			r1 := u.Passwd

			// run again
			u.SetPassword(pass)
			r2 := u.Passwd

			assert.NotEqual(t, r1, r2)
			assert.True(t, u.ValidatePassword(pass))
		}
	}
}

func BenchmarkHashPassword(b *testing.B) {
	// BenchmarkHashPassword ensures that it takes a reasonable amount of time
	// to hash a password - in order to protect from brute-force attacks.
	pass := "password1337"
	u := &User{Passwd: pass}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		u.SetPassword(pass)
	}
}

func TestNewGitSig(t *testing.T) {
	users := make([]*User, 0, 20)
	err := db.GetEngine(db.DefaultContext).Find(&users)
	assert.NoError(t, err)

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
	err := db.GetEngine(db.DefaultContext).Find(&users)
	assert.NoError(t, err)

	for _, user := range users {
		displayName := user.DisplayName()
		assert.Equal(t, strings.TrimSpace(displayName), displayName)
		if len(strings.TrimSpace(user.FullName)) == 0 {
			assert.Equal(t, user.Name, displayName)
		}
		assert.NotEqual(t, len(strings.TrimSpace(displayName)), 0)
	}
}

func TestCreateUserInvalidEmail(t *testing.T) {
	user := &User{
		Name:               "GiteaBot",
		Email:              "GiteaBot@gitea.io\r\n",
		Passwd:             ";p['////..-++']",
		IsAdmin:            false,
		Theme:              setting.UI.DefaultTheme,
		MustChangePassword: false,
	}

	err := CreateUser(user)
	assert.Error(t, err)
	assert.True(t, IsErrEmailInvalid(err))
}

func TestGetUserIDsByNames(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// ignore non existing
	IDs, err := GetUserIDsByNames([]string{"user1", "user2", "none_existing_user"}, true)
	assert.NoError(t, err)
	assert.Equal(t, []int64{1, 2}, IDs)

	// ignore non existing
	IDs, err = GetUserIDsByNames([]string{"user1", "do_not_exist"}, false)
	assert.Error(t, err)
	assert.Equal(t, []int64(nil), IDs)
}

func TestGetMaileableUsersByIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	results, err := GetMaileableUsersByIDs([]int64{1, 4}, false)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	if len(results) > 1 {
		assert.Equal(t, results[0].ID, 1)
	}

	results, err = GetMaileableUsersByIDs([]int64{1, 4}, true)
	assert.NoError(t, err)
	assert.Len(t, results, 2)
	if len(results) > 2 {
		assert.Equal(t, results[0].ID, 1)
		assert.Equal(t, results[1].ID, 4)
	}
}

func TestUpdateUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)

	user.KeepActivityPrivate = true
	assert.NoError(t, UpdateUser(user, false))
	user = unittest.AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	assert.True(t, user.KeepActivityPrivate)

	setting.Service.AllowedUserVisibilityModesSlice = []bool{true, false, false}
	user.KeepActivityPrivate = false
	user.Visibility = structs.VisibleTypePrivate
	assert.Error(t, UpdateUser(user, false))
	user = unittest.AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	assert.True(t, user.KeepActivityPrivate)

	user.Email = "no mail@mail.org"
	assert.Error(t, UpdateUser(user, true))
}

func TestNewUserRedirect(t *testing.T) {
	// redirect to a completely new name
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &User{ID: 1}).(*User)
	assert.NoError(t, NewUserRedirect(db.DefaultContext, user.ID, user.Name, "newusername"))

	unittest.AssertExistsAndLoadBean(t, &Redirect{
		LowerName:      user.LowerName,
		RedirectUserID: user.ID,
	})
	unittest.AssertExistsAndLoadBean(t, &Redirect{
		LowerName:      "olduser1",
		RedirectUserID: user.ID,
	})
}

func TestNewUserRedirect2(t *testing.T) {
	// redirect to previously used name
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &User{ID: 1}).(*User)
	assert.NoError(t, NewUserRedirect(db.DefaultContext, user.ID, user.Name, "olduser1"))

	unittest.AssertExistsAndLoadBean(t, &Redirect{
		LowerName:      user.LowerName,
		RedirectUserID: user.ID,
	})
	unittest.AssertNotExistsBean(t, &Redirect{
		LowerName:      "olduser1",
		RedirectUserID: user.ID,
	})
}

func TestNewUserRedirect3(t *testing.T) {
	// redirect for a previously-unredirected user
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	assert.NoError(t, NewUserRedirect(db.DefaultContext, user.ID, user.Name, "newusername"))

	unittest.AssertExistsAndLoadBean(t, &Redirect{
		LowerName:      user.LowerName,
		RedirectUserID: user.ID,
	})
}

func TestGetUserByOpenID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	_, err := GetUserByOpenID("https://unknown")
	if assert.Error(t, err) {
		assert.True(t, IsErrUserNotExist(err))
	}

	user, err := GetUserByOpenID("https://user1.domain1.tld")
	if assert.NoError(t, err) {
		assert.Equal(t, int64(1), user.ID)
	}

	user, err = GetUserByOpenID("https://domain1.tld/user2/")
	if assert.NoError(t, err) {
		assert.Equal(t, int64(2), user.ID)
	}
}
