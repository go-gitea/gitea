// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"testing"
	"time"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/auth/password/hash"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestOAuth2Application_LoadUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	app := unittest.AssertExistsAndLoadBean(t, &auth.OAuth2Application{ID: 1})
	user, err := user_model.GetUserByID(db.DefaultContext, app.UID)
	assert.NoError(t, err)
	assert.NotNil(t, user)
}

func TestGetUserEmailsByNames(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// ignore none active user email
	assert.ElementsMatch(t, []string{"user8@example.com"}, user_model.GetUserEmailsByNames(db.DefaultContext, []string{"user8", "user9"}))
	assert.ElementsMatch(t, []string{"user8@example.com", "user5@example.com"}, user_model.GetUserEmailsByNames(db.DefaultContext, []string{"user8", "user5"}))

	assert.ElementsMatch(t, []string{"user8@example.com"}, user_model.GetUserEmailsByNames(db.DefaultContext, []string{"user8", "org7"}))
}

func TestCanCreateOrganization(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	assert.True(t, admin.CanCreateOrganization())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
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
		users, _, err := user_model.SearchUsers(db.DefaultContext, opts)
		assert.NoError(t, err)
		cassText := fmt.Sprintf("ids: %v, opts: %v", expectedUserOrOrgIDs, opts)
		if assert.Len(t, users, len(expectedUserOrOrgIDs), "case: %s", cassText) {
			for i, expectedID := range expectedUserOrOrgIDs {
				assert.EqualValues(t, expectedID, users[i].ID, "case: %s", cassText)
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
		[]int64{26, 41})

	testOrgSuccess(&user_model.SearchUserOptions{ListOptions: db.ListOptions{Page: 5, PageSize: 2}},
		[]int64{})

	// test users
	testUserSuccess := func(opts *user_model.SearchUserOptions, expectedUserIDs []int64) {
		opts.Type = user_model.UserTypeIndividual
		testSuccess(opts, expectedUserIDs)
	}

	testUserSuccess(&user_model.SearchUserOptions{OrderBy: "id ASC", ListOptions: db.ListOptions{Page: 1}},
		[]int64{1, 2, 4, 5, 8, 9, 10, 11, 12, 13, 14, 15, 16, 18, 20, 21, 24, 27, 28, 29, 30, 32, 34, 37, 38, 39, 40})

	testUserSuccess(&user_model.SearchUserOptions{ListOptions: db.ListOptions{Page: 1}, IsActive: optional.Some(false)},
		[]int64{9})

	testUserSuccess(&user_model.SearchUserOptions{OrderBy: "id ASC", ListOptions: db.ListOptions{Page: 1}, IsActive: optional.Some(true)},
		[]int64{1, 2, 4, 5, 8, 10, 11, 12, 13, 14, 15, 16, 18, 20, 21, 24, 27, 28, 29, 30, 32, 34, 37, 38, 39, 40})

	testUserSuccess(&user_model.SearchUserOptions{Keyword: "user1", OrderBy: "id ASC", ListOptions: db.ListOptions{Page: 1}, IsActive: optional.Some(true)},
		[]int64{1, 10, 11, 12, 13, 14, 15, 16, 18})

	// order by name asc default
	testUserSuccess(&user_model.SearchUserOptions{Keyword: "user1", ListOptions: db.ListOptions{Page: 1}, IsActive: optional.Some(true)},
		[]int64{1, 10, 11, 12, 13, 14, 15, 16, 18})

	testUserSuccess(&user_model.SearchUserOptions{ListOptions: db.ListOptions{Page: 1}, IsAdmin: optional.Some(true)},
		[]int64{1})

	testUserSuccess(&user_model.SearchUserOptions{ListOptions: db.ListOptions{Page: 1}, IsRestricted: optional.Some(true)},
		[]int64{29})

	testUserSuccess(&user_model.SearchUserOptions{ListOptions: db.ListOptions{Page: 1}, IsProhibitLogin: optional.Some(true)},
		[]int64{37})

	testUserSuccess(&user_model.SearchUserOptions{ListOptions: db.ListOptions{Page: 1}, IsTwoFactorEnabled: optional.Some(true)},
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
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: test.userID})
		assert.Equal(t, test.expected, user.EmailNotificationsPreference)
	}
}

func TestHashPasswordDeterministic(t *testing.T) {
	b := make([]byte, 16)
	u := &user_model.User{}
	algos := hash.RecommendedHashAlgorithms
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

	err := user_model.CreateUser(db.DefaultContext, user)
	assert.Error(t, err)
	assert.True(t, user_model.IsErrEmailCharIsNotSupported(err))
}

func TestCreateUserEmailAlreadyUsed(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// add new user with user2's email
	user.Name = "testuser"
	user.LowerName = strings.ToLower(user.Name)
	user.ID = 0
	err := user_model.CreateUser(db.DefaultContext, user)
	assert.Error(t, err)
	assert.True(t, user_model.IsErrEmailAlreadyUsed(err))
}

func TestCreateUserCustomTimestamps(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// Add new user with a custom creation timestamp.
	var creationTimestamp timeutil.TimeStamp = 12345
	user.Name = "testuser"
	user.LowerName = strings.ToLower(user.Name)
	user.ID = 0
	user.Email = "unique@example.com"
	user.CreatedUnix = creationTimestamp
	err := user_model.CreateUser(db.DefaultContext, user)
	assert.NoError(t, err)

	fetched, err := user_model.GetUserByID(context.Background(), user.ID)
	assert.NoError(t, err)
	assert.Equal(t, creationTimestamp, fetched.CreatedUnix)
	assert.Equal(t, creationTimestamp, fetched.UpdatedUnix)
}

func TestCreateUserWithoutCustomTimestamps(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// There is no way to use a mocked time for the XORM auto-time functionality,
	// so use the real clock to approximate the expected timestamp.
	timestampStart := time.Now().Unix()

	// Add new user without a custom creation timestamp.
	user.Name = "Testuser"
	user.LowerName = strings.ToLower(user.Name)
	user.ID = 0
	user.Email = "unique@example.com"
	user.CreatedUnix = 0
	user.UpdatedUnix = 0
	err := user_model.CreateUser(db.DefaultContext, user)
	assert.NoError(t, err)

	timestampEnd := time.Now().Unix()

	fetched, err := user_model.GetUserByID(context.Background(), user.ID)
	assert.NoError(t, err)

	assert.LessOrEqual(t, timestampStart, fetched.CreatedUnix)
	assert.LessOrEqual(t, fetched.CreatedUnix, timestampEnd)

	assert.LessOrEqual(t, timestampStart, fetched.UpdatedUnix)
	assert.LessOrEqual(t, fetched.UpdatedUnix, timestampEnd)
}

func TestGetUserIDsByNames(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// ignore non existing
	IDs, err := user_model.GetUserIDsByNames(db.DefaultContext, []string{"user1", "user2", "none_existing_user"}, true)
	assert.NoError(t, err)
	assert.Equal(t, []int64{1, 2}, IDs)

	// ignore non existing
	IDs, err = user_model.GetUserIDsByNames(db.DefaultContext, []string{"user1", "do_not_exist"}, false)
	assert.Error(t, err)
	assert.Equal(t, []int64(nil), IDs)
}

func TestGetMaileableUsersByIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	results, err := user_model.GetMaileableUsersByIDs(db.DefaultContext, []int64{1, 4}, false)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	if len(results) > 1 {
		assert.Equal(t, results[0].ID, 1)
	}

	results, err = user_model.GetMaileableUsersByIDs(db.DefaultContext, []int64{1, 4}, true)
	assert.NoError(t, err)
	assert.Len(t, results, 2)
	if len(results) > 2 {
		assert.Equal(t, results[0].ID, 1)
		assert.Equal(t, results[1].ID, 4)
	}
}

func TestNewUserRedirect(t *testing.T) {
	// redirect to a completely new name
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
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

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
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

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	assert.NoError(t, user_model.NewUserRedirect(db.DefaultContext, user.ID, user.Name, "newusername"))

	unittest.AssertExistsAndLoadBean(t, &user_model.Redirect{
		LowerName:      user.LowerName,
		RedirectUserID: user.ID,
	})
}

func TestGetUserByOpenID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	_, err := user_model.GetUserByOpenID(db.DefaultContext, "https://unknown")
	if assert.Error(t, err) {
		assert.True(t, user_model.IsErrUserNotExist(err))
	}

	user, err := user_model.GetUserByOpenID(db.DefaultContext, "https://user1.domain1.tld")
	if assert.NoError(t, err) {
		assert.Equal(t, int64(1), user.ID)
	}

	user, err = user_model.GetUserByOpenID(db.DefaultContext, "https://domain1.tld/user2/")
	if assert.NoError(t, err) {
		assert.Equal(t, int64(2), user.ID)
	}
}

func TestFollowUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(follower, followed *user_model.User) {
		assert.NoError(t, user_model.FollowUser(db.DefaultContext, follower, followed))
		unittest.AssertExistsAndLoadBean(t, &user_model.Follow{UserID: follower.ID, FollowID: followed.ID})
	}

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	testSuccess(user4, user2)
	testSuccess(user5, user2)

	assert.NoError(t, user_model.FollowUser(db.DefaultContext, user2, user2))

	unittest.CheckConsistencyFor(t, &user_model.User{})
}

func TestUnfollowUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(followerID, followedID int64) {
		assert.NoError(t, user_model.UnfollowUser(db.DefaultContext, followerID, followedID))
		unittest.AssertNotExistsBean(t, &user_model.Follow{UserID: followerID, FollowID: followedID})
	}
	testSuccess(4, 2)
	testSuccess(5, 2)
	testSuccess(2, 2)

	unittest.CheckConsistencyFor(t, &user_model.User{})
}

func TestIsUserVisibleToViewer(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})   // admin, public
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})   // normal, public
	user20 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 20}) // public, same team as user31
	user29 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 29}) // public, is restricted
	user31 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 31}) // private, same team as user20
	user33 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 33}) // limited, follows 31

	test := func(u, viewer *user_model.User, expected bool) {
		name := func(u *user_model.User) string {
			if u == nil {
				return "<nil>"
			}
			return u.Name
		}
		assert.Equal(t, expected, user_model.IsUserVisibleToViewer(db.DefaultContext, u, viewer), "user %v should be visible to viewer %v: %v", name(u), name(viewer), expected)
	}

	// admin viewer
	test(user1, user1, true)
	test(user20, user1, true)
	test(user31, user1, true)
	test(user33, user1, true)

	// non admin viewer
	test(user4, user4, true)
	test(user20, user4, true)
	test(user31, user4, false)
	test(user33, user4, true)
	test(user4, nil, true)

	// public user
	test(user4, user20, true)
	test(user4, user31, true)
	test(user4, user33, true)

	// limited user
	test(user33, user33, true)
	test(user33, user4, true)
	test(user33, user29, false)
	test(user33, nil, false)

	// private user
	test(user31, user31, true)
	test(user31, user4, false)
	test(user31, user20, true)
	test(user31, user29, false)
	test(user31, user33, true)
	test(user31, nil, false)
}

func Test_ValidateUser(t *testing.T) {
	oldSetting := setting.Service.AllowedUserVisibilityModesSlice
	defer func() {
		setting.Service.AllowedUserVisibilityModesSlice = oldSetting
	}()
	setting.Service.AllowedUserVisibilityModesSlice = []bool{true, false, true}
	kases := map[*user_model.User]bool{
		{ID: 1, Visibility: structs.VisibleTypePublic}:  true,
		{ID: 2, Visibility: structs.VisibleTypeLimited}: false,
		{ID: 2, Visibility: structs.VisibleTypePrivate}: true,
	}
	for kase, expected := range kases {
		assert.EqualValues(t, expected, nil == user_model.ValidateUser(kase), fmt.Sprintf("case: %+v", kase))
	}
}

func Test_NormalizeUserFromEmail(t *testing.T) {
	testCases := []struct {
		Input             string
		Expected          string
		IsNormalizedValid bool
	}{
		{"name@example.com", "name", true},
		{"test'`´name", "testname", true},
		{"Sinéad.O'Connor", "Sinead.OConnor", true},
		{"Æsir", "AEsir", true},
		{"éé", "ee", true}, // \u00e9\u0065\u0301
		{"Awareness Hub", "Awareness-Hub", true},
		{"double__underscore", "double__underscore", false}, // We should consider squashing double non-alpha characters
		{".bad.", ".bad.", false},
		{"new😀user", "new😀user", false}, // No plans to support
		{`"quoted"`, `"quoted"`, false}, // No plans to support
	}
	for _, testCase := range testCases {
		normalizedName, err := user_model.NormalizeUserName(testCase.Input)
		assert.NoError(t, err)
		assert.EqualValues(t, testCase.Expected, normalizedName)
		if testCase.IsNormalizedValid {
			assert.NoError(t, user_model.IsUsableUsername(normalizedName))
		} else {
			assert.Error(t, user_model.IsUsableUsername(normalizedName))
		}
	}
}

func TestDisabledUserFeatures(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testValues := container.SetOf(setting.UserFeatureDeletion,
		setting.UserFeatureManageSSHKeys,
		setting.UserFeatureManageGPGKeys)

	oldSetting := setting.Admin.ExternalUserDisableFeatures
	defer func() {
		setting.Admin.ExternalUserDisableFeatures = oldSetting
	}()
	setting.Admin.ExternalUserDisableFeatures = testValues

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	assert.Len(t, setting.Admin.UserDisabledFeatures.Values(), 0)

	// no features should be disabled with a plain login type
	assert.LessOrEqual(t, user.LoginType, auth.Plain)
	assert.Len(t, user_model.DisabledFeaturesWithLoginType(user).Values(), 0)
	for _, f := range testValues.Values() {
		assert.False(t, user_model.IsFeatureDisabledWithLoginType(user, f))
	}

	// check disabled features with external login type
	user.LoginType = auth.OAuth2

	// all features should be disabled
	assert.NotEmpty(t, user_model.DisabledFeaturesWithLoginType(user).Values())
	for _, f := range testValues.Values() {
		assert.True(t, user_model.IsFeatureDisabledWithLoginType(user, f))
	}
}
