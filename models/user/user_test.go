// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user_test

import (
	"math/rand"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestOAuth2Application_LoadUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	app := unittest.AssertExistsAndLoadBean(t, &auth.OAuth2Application{ID: 1}).(*auth.OAuth2Application)
	user, err := user_model.GetUserByID(app.UID)
	assert.NoError(t, err)
	assert.NotNil(t, user)
}

func TestGetUserEmailsByNames(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// ignore none active user email
	assert.Equal(t, []string{"user8@example.com"}, user_model.GetUserEmailsByNames(db.DefaultContext, []string{"user8", "user9"}))
	assert.Equal(t, []string{"user8@example.com", "user5@example.com"}, user_model.GetUserEmailsByNames(db.DefaultContext, []string{"user8", "user5"}))

	assert.Equal(t, []string{"user8@example.com"}, user_model.GetUserEmailsByNames(db.DefaultContext, []string{"user8", "user7"}))
}

func TestCanCreateOrganization(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)
	assert.True(t, admin.CanCreateOrganization())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
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
	testSuccess := func(opts *user_model.SearchUserOptions, expectedUserOrOrgIDs []int64) {
		users, _, err := user_model.SearchUsers(opts)
		assert.NoError(t, err)
		if assert.Len(t, users, len(expectedUserOrOrgIDs), opts) {
			for i, expectedID := range expectedUserOrOrgIDs {
				assert.EqualValues(t, expectedID, users[i].ID)
			}
		}
	}

	// test orgs
	testOrgSuccess := func(opts *user_model.SearchUserOptions, expectedOrgIDs []int64) {
		opts.Type = user_model.UserTypeOrganization
		testSuccess(opts, expectedOrgIDs)
	}

	testOrgSuccess(&user_model.SearchUserOptions{OrderBy: "id ASC", ListOptions: db.ListOptions{Page: 1, PageSize: 2}},
		[]int64{3, 6})

	testOrgSuccess(&user_model.SearchUserOptions{OrderBy: "id ASC", ListOptions: db.ListOptions{Page: 2, PageSize: 2}},
		[]int64{7, 17})

	testOrgSuccess(&user_model.SearchUserOptions{OrderBy: "id ASC", ListOptions: db.ListOptions{Page: 3, PageSize: 2}},
		[]int64{19, 25})

	testOrgSuccess(&user_model.SearchUserOptions{OrderBy: "id ASC", ListOptions: db.ListOptions{Page: 4, PageSize: 2}},
		[]int64{26})

	testOrgSuccess(&user_model.SearchUserOptions{ListOptions: db.ListOptions{Page: 5, PageSize: 2}},
		[]int64{})

	// test users
	testUserSuccess := func(opts *user_model.SearchUserOptions, expectedUserIDs []int64) {
		opts.Type = user_model.UserTypeIndividual
		testSuccess(opts, expectedUserIDs)
	}

	testUserSuccess(&user_model.SearchUserOptions{OrderBy: "id ASC", ListOptions: db.ListOptions{Page: 1}},
		[]int64{1, 2, 4, 5, 8, 9, 10, 11, 12, 13, 14, 15, 16, 18, 20, 21, 24, 27, 28, 29, 30, 32})

	testUserSuccess(&user_model.SearchUserOptions{ListOptions: db.ListOptions{Page: 1}, IsActive: util.OptionalBoolFalse},
		[]int64{9})

	testUserSuccess(&user_model.SearchUserOptions{OrderBy: "id ASC", ListOptions: db.ListOptions{Page: 1}, IsActive: util.OptionalBoolTrue},
		[]int64{1, 2, 4, 5, 8, 10, 11, 12, 13, 14, 15, 16, 18, 20, 21, 24, 28, 29, 30, 32})

	testUserSuccess(&user_model.SearchUserOptions{Keyword: "user1", OrderBy: "id ASC", ListOptions: db.ListOptions{Page: 1}, IsActive: util.OptionalBoolTrue},
		[]int64{1, 10, 11, 12, 13, 14, 15, 16, 18})

	// order by name asc default
	testUserSuccess(&user_model.SearchUserOptions{Keyword: "user1", ListOptions: db.ListOptions{Page: 1}, IsActive: util.OptionalBoolTrue},
		[]int64{1, 10, 11, 12, 13, 14, 15, 16, 18})

	testUserSuccess(&user_model.SearchUserOptions{ListOptions: db.ListOptions{Page: 1}, IsAdmin: util.OptionalBoolTrue},
		[]int64{1})

	testUserSuccess(&user_model.SearchUserOptions{ListOptions: db.ListOptions{Page: 1}, IsRestricted: util.OptionalBoolTrue},
		[]int64{29, 30})

	testUserSuccess(&user_model.SearchUserOptions{ListOptions: db.ListOptions{Page: 1}, IsProhibitLogin: util.OptionalBoolTrue},
		[]int64{30})

	testUserSuccess(&user_model.SearchUserOptions{ListOptions: db.ListOptions{Page: 1}, IsTwoFactorEnabled: util.OptionalBoolTrue},
		[]int64{24})
}

func TestEmailNotificationPreferences(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	for _, test := range []struct {
		expected string
		userID   int64
	}{
		{user_model.EmailNotificationsEnabled, 1},
		{user_model.EmailNotificationsEnabled, 2},
		{user_model.EmailNotificationsOnMention, 3},
		{user_model.EmailNotificationsOnMention, 4},
		{user_model.EmailNotificationsEnabled, 5},
		{user_model.EmailNotificationsEnabled, 6},
		{user_model.EmailNotificationsDisabled, 7},
		{user_model.EmailNotificationsEnabled, 8},
		{user_model.EmailNotificationsOnMention, 9},
	} {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: test.userID}).(*user_model.User)
		assert.Equal(t, test.expected, user.EmailNotifications())

		// Try all possible settings
		assert.NoError(t, user_model.SetEmailNotifications(user, user_model.EmailNotificationsEnabled))
		assert.Equal(t, user_model.EmailNotificationsEnabled, user.EmailNotifications())

		assert.NoError(t, user_model.SetEmailNotifications(user, user_model.EmailNotificationsOnMention))
		assert.Equal(t, user_model.EmailNotificationsOnMention, user.EmailNotifications())

		assert.NoError(t, user_model.SetEmailNotifications(user, user_model.EmailNotificationsDisabled))
		assert.Equal(t, user_model.EmailNotificationsDisabled, user.EmailNotifications())

		assert.NoError(t, user_model.SetEmailNotifications(user, user_model.EmailNotificationsAndYourOwn))
		assert.Equal(t, user_model.EmailNotificationsAndYourOwn, user.EmailNotifications())
	}
}

func TestHashPasswordDeterministic(t *testing.T) {
	b := make([]byte, 16)
	u := &user_model.User{}
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
	u := &user_model.User{Passwd: pass}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		u.SetPassword(pass)
	}
}

func TestNewGitSig(t *testing.T) {
	users := make([]*user_model.User, 0, 20)
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
	users := make([]*user_model.User, 0, 20)
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
	user := &user_model.User{
		Name:               "GiteaBot",
		Email:              "GiteaBot@gitea.io\r\n",
		Passwd:             ";p['////..-++']",
		IsAdmin:            false,
		Theme:              setting.UI.DefaultTheme,
		MustChangePassword: false,
	}

	err := user_model.CreateUser(user)
	assert.Error(t, err)
	assert.True(t, user_model.IsErrEmailCharIsNotSupported(err))
}

func TestCreateUserEmailAlreadyUsed(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)

	// add new user with user2's email
	user.Name = "testuser"
	user.LowerName = strings.ToLower(user.Name)
	user.ID = 0
	err := user_model.CreateUser(user)
	assert.Error(t, err)
	assert.True(t, user_model.IsErrEmailAlreadyUsed(err))
}

func TestGetUserIDsByNames(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// ignore non existing
	IDs, err := user_model.GetUserIDsByNames([]string{"user1", "user2", "none_existing_user"}, true)
	assert.NoError(t, err)
	assert.Equal(t, []int64{1, 2}, IDs)

	// ignore non existing
	IDs, err = user_model.GetUserIDsByNames([]string{"user1", "do_not_exist"}, false)
	assert.Error(t, err)
	assert.Equal(t, []int64(nil), IDs)
}

func TestGetMaileableUsersByIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	results, err := user_model.GetMaileableUsersByIDs([]int64{1, 4}, false)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	if len(results) > 1 {
		assert.Equal(t, results[0].ID, 1)
	}

	results, err = user_model.GetMaileableUsersByIDs([]int64{1, 4}, true)
	assert.NoError(t, err)
	assert.Len(t, results, 2)
	if len(results) > 2 {
		assert.Equal(t, results[0].ID, 1)
		assert.Equal(t, results[1].ID, 4)
	}
}

func TestUpdateUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)

	user.KeepActivityPrivate = true
	assert.NoError(t, user_model.UpdateUser(db.DefaultContext, user, false))
	user = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	assert.True(t, user.KeepActivityPrivate)

	setting.Service.AllowedUserVisibilityModesSlice = []bool{true, false, false}
	user.KeepActivityPrivate = false
	user.Visibility = structs.VisibleTypePrivate
	assert.Error(t, user_model.UpdateUser(db.DefaultContext, user, false))
	user = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	assert.True(t, user.KeepActivityPrivate)

	user.Email = "no mail@mail.org"
	assert.Error(t, user_model.UpdateUser(db.DefaultContext, user, true))
}

func TestNewUserRedirect(t *testing.T) {
	// redirect to a completely new name
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)
	assert.NoError(t, user_model.NewUserRedirect(db.DefaultContext, user.ID, user.Name, "newusername"))

	unittest.AssertExistsAndLoadBean(t, &user_model.Redirect{
		LowerName:      user.LowerName,
		RedirectUserID: user.ID,
	})
	unittest.AssertExistsAndLoadBean(t, &user_model.Redirect{
		LowerName:      "olduser1",
		RedirectUserID: user.ID,
	})
}

func TestNewUserRedirect2(t *testing.T) {
	// redirect to previously used name
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)
	assert.NoError(t, user_model.NewUserRedirect(db.DefaultContext, user.ID, user.Name, "olduser1"))

	unittest.AssertExistsAndLoadBean(t, &user_model.Redirect{
		LowerName:      user.LowerName,
		RedirectUserID: user.ID,
	})
	unittest.AssertNotExistsBean(t, &user_model.Redirect{
		LowerName:      "olduser1",
		RedirectUserID: user.ID,
	})
}

func TestNewUserRedirect3(t *testing.T) {
	// redirect for a previously-unredirected user
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	assert.NoError(t, user_model.NewUserRedirect(db.DefaultContext, user.ID, user.Name, "newusername"))

	unittest.AssertExistsAndLoadBean(t, &user_model.Redirect{
		LowerName:      user.LowerName,
		RedirectUserID: user.ID,
	})
}

func TestGetUserByOpenID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	_, err := user_model.GetUserByOpenID("https://unknown")
	if assert.Error(t, err) {
		assert.True(t, user_model.IsErrUserNotExist(err))
	}

	user, err := user_model.GetUserByOpenID("https://user1.domain1.tld")
	if assert.NoError(t, err) {
		assert.Equal(t, int64(1), user.ID)
	}

	user, err = user_model.GetUserByOpenID("https://domain1.tld/user2/")
	if assert.NoError(t, err) {
		assert.Equal(t, int64(2), user.ID)
	}
}

func TestFollowUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(followerID, followedID int64) {
		assert.NoError(t, user_model.FollowUser(followerID, followedID))
		unittest.AssertExistsAndLoadBean(t, &user_model.Follow{UserID: followerID, FollowID: followedID})
	}
	testSuccess(4, 2)
	testSuccess(5, 2)

	assert.NoError(t, user_model.FollowUser(2, 2))

	unittest.CheckConsistencyFor(t, &user_model.User{})
}

func TestUnfollowUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(followerID, followedID int64) {
		assert.NoError(t, user_model.UnfollowUser(followerID, followedID))
		unittest.AssertNotExistsBean(t, &user_model.Follow{UserID: followerID, FollowID: followedID})
	}
	testSuccess(4, 2)
	testSuccess(5, 2)
	testSuccess(2, 2)

	unittest.CheckConsistencyFor(t, &user_model.User{})
}
