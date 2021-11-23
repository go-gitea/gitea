// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/login"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
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
	user, err := GetUserByID(uid)
	assert.NoError(t, err)
	is, err := IsPublicMembership(orgID, user.ID)
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
	user, err := GetUserByID(uid)
	assert.NoError(t, err)
	is, err := IsOrganizationOwner(orgID, user.ID)
	assert.NoError(t, err)
	assert.Equal(t, expected, is)
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

func TestGetOrgRepositoryIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user2 := unittest.AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	user4 := unittest.AssertExistsAndLoadBean(t, &User{ID: 4}).(*User)
	user5 := unittest.AssertExistsAndLoadBean(t, &User{ID: 5}).(*User)

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
	assert.True(t, user_model.IsErrEmailInvalid(err))
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

func TestAddLdapSSHPublicKeys(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	s := &login.Source{ID: 1}

	testCases := []struct {
		keyString   string
		number      int
		keyContents []string
	}{
		{
			keyString: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC4cn+iXnA4KvcQYSV88vGn0Yi91vG47t1P7okprVmhNTkipNRIHWr6WdCO4VDr/cvsRkuVJAsLO2enwjGWWueOO6BodiBgyAOZ/5t5nJNMCNuLGT5UIo/RI1b0WRQwxEZTRjt6mFNw6lH14wRd8ulsr9toSWBPMOGWoYs1PDeDL0JuTjL+tr1SZi/EyxCngpYszKdXllJEHyI79KQgeD0Vt3pTrkbNVTOEcCNqZePSVmUH8X8Vhugz3bnE0/iE9Pb5fkWO9c4AnM1FgI/8Bvp27Fw2ShryIXuR6kKvUqhVMTuOSDHwu6A8jLE5Owt3GAYugDpDYuwTVNGrHLXKpPzrGGPE/jPmaLCMZcsdkec95dYeU3zKODEm8UQZFhmJmDeWVJ36nGrGZHL4J5aTTaeFUJmmXDaJYiJ+K2/ioKgXqnXvltu0A9R8/LGy4nrTJRr4JMLuJFoUXvGm1gXQ70w2LSpk6yl71RNC0hCtsBe8BP8IhYCM0EP5jh7eCMQZNvM= nocomment\n",
			number:    1,
			keyContents: []string{
				"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC4cn+iXnA4KvcQYSV88vGn0Yi91vG47t1P7okprVmhNTkipNRIHWr6WdCO4VDr/cvsRkuVJAsLO2enwjGWWueOO6BodiBgyAOZ/5t5nJNMCNuLGT5UIo/RI1b0WRQwxEZTRjt6mFNw6lH14wRd8ulsr9toSWBPMOGWoYs1PDeDL0JuTjL+tr1SZi/EyxCngpYszKdXllJEHyI79KQgeD0Vt3pTrkbNVTOEcCNqZePSVmUH8X8Vhugz3bnE0/iE9Pb5fkWO9c4AnM1FgI/8Bvp27Fw2ShryIXuR6kKvUqhVMTuOSDHwu6A8jLE5Owt3GAYugDpDYuwTVNGrHLXKpPzrGGPE/jPmaLCMZcsdkec95dYeU3zKODEm8UQZFhmJmDeWVJ36nGrGZHL4J5aTTaeFUJmmXDaJYiJ+K2/ioKgXqnXvltu0A9R8/LGy4nrTJRr4JMLuJFoUXvGm1gXQ70w2LSpk6yl71RNC0hCtsBe8BP8IhYCM0EP5jh7eCMQZNvM=",
			},
		},
		{
			keyString: `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC4cn+iXnA4KvcQYSV88vGn0Yi91vG47t1P7okprVmhNTkipNRIHWr6WdCO4VDr/cvsRkuVJAsLO2enwjGWWueOO6BodiBgyAOZ/5t5nJNMCNuLGT5UIo/RI1b0WRQwxEZTRjt6mFNw6lH14wRd8ulsr9toSWBPMOGWoYs1PDeDL0JuTjL+tr1SZi/EyxCngpYszKdXllJEHyI79KQgeD0Vt3pTrkbNVTOEcCNqZePSVmUH8X8Vhugz3bnE0/iE9Pb5fkWO9c4AnM1FgI/8Bvp27Fw2ShryIXuR6kKvUqhVMTuOSDHwu6A8jLE5Owt3GAYugDpDYuwTVNGrHLXKpPzrGGPE/jPmaLCMZcsdkec95dYeU3zKODEm8UQZFhmJmDeWVJ36nGrGZHL4J5aTTaeFUJmmXDaJYiJ+K2/ioKgXqnXvltu0A9R8/LGy4nrTJRr4JMLuJFoUXvGm1gXQ70w2LSpk6yl71RNC0hCtsBe8BP8IhYCM0EP5jh7eCMQZNvM= nocomment
ssh-dss AAAAB3NzaC1kc3MAAACBAOChCC7lf6Uo9n7BmZ6M8St19PZf4Tn59NriyboW2x/DZuYAz3ibZ2OkQ3S0SqDIa0HXSEJ1zaExQdmbO+Ux/wsytWZmCczWOVsaszBZSl90q8UnWlSH6P+/YA+RWJm5SFtuV9PtGIhyZgoNuz5kBQ7K139wuQsecdKktISwTakzAAAAFQCzKsO2JhNKlL+wwwLGOcLffoAmkwAAAIBpK7/3xvduajLBD/9vASqBQIHrgK2J+wiQnIb/Wzy0UsVmvfn8A+udRbBo+csM8xrSnlnlJnjkJS3qiM5g+eTwsLIV1IdKPEwmwB+VcP53Cw6lSyWyJcvhFb0N6s08NZysLzvj0N+ZC/FnhKTLzIyMtkHf/IrPCwlM+pV/M/96YgAAAIEAqQcGn9CKgzgPaguIZooTAOQdvBLMI5y0bQjOW6734XOpqQGf/Kra90wpoasLKZjSYKNPjE+FRUOrStLrxcNs4BeVKhy2PYTRnybfYVk1/dmKgH6P1YSRONsGKvTsH6c5IyCRG0ncCgYeF8tXppyd642982daopE7zQ/NPAnJfag= nocomment`,
			number: 2,
			keyContents: []string{
				"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC4cn+iXnA4KvcQYSV88vGn0Yi91vG47t1P7okprVmhNTkipNRIHWr6WdCO4VDr/cvsRkuVJAsLO2enwjGWWueOO6BodiBgyAOZ/5t5nJNMCNuLGT5UIo/RI1b0WRQwxEZTRjt6mFNw6lH14wRd8ulsr9toSWBPMOGWoYs1PDeDL0JuTjL+tr1SZi/EyxCngpYszKdXllJEHyI79KQgeD0Vt3pTrkbNVTOEcCNqZePSVmUH8X8Vhugz3bnE0/iE9Pb5fkWO9c4AnM1FgI/8Bvp27Fw2ShryIXuR6kKvUqhVMTuOSDHwu6A8jLE5Owt3GAYugDpDYuwTVNGrHLXKpPzrGGPE/jPmaLCMZcsdkec95dYeU3zKODEm8UQZFhmJmDeWVJ36nGrGZHL4J5aTTaeFUJmmXDaJYiJ+K2/ioKgXqnXvltu0A9R8/LGy4nrTJRr4JMLuJFoUXvGm1gXQ70w2LSpk6yl71RNC0hCtsBe8BP8IhYCM0EP5jh7eCMQZNvM=",
				"ssh-dss AAAAB3NzaC1kc3MAAACBAOChCC7lf6Uo9n7BmZ6M8St19PZf4Tn59NriyboW2x/DZuYAz3ibZ2OkQ3S0SqDIa0HXSEJ1zaExQdmbO+Ux/wsytWZmCczWOVsaszBZSl90q8UnWlSH6P+/YA+RWJm5SFtuV9PtGIhyZgoNuz5kBQ7K139wuQsecdKktISwTakzAAAAFQCzKsO2JhNKlL+wwwLGOcLffoAmkwAAAIBpK7/3xvduajLBD/9vASqBQIHrgK2J+wiQnIb/Wzy0UsVmvfn8A+udRbBo+csM8xrSnlnlJnjkJS3qiM5g+eTwsLIV1IdKPEwmwB+VcP53Cw6lSyWyJcvhFb0N6s08NZysLzvj0N+ZC/FnhKTLzIyMtkHf/IrPCwlM+pV/M/96YgAAAIEAqQcGn9CKgzgPaguIZooTAOQdvBLMI5y0bQjOW6734XOpqQGf/Kra90wpoasLKZjSYKNPjE+FRUOrStLrxcNs4BeVKhy2PYTRnybfYVk1/dmKgH6P1YSRONsGKvTsH6c5IyCRG0ncCgYeF8tXppyd642982daopE7zQ/NPAnJfag=",
			},
		},
		{
			keyString: `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC4cn+iXnA4KvcQYSV88vGn0Yi91vG47t1P7okprVmhNTkipNRIHWr6WdCO4VDr/cvsRkuVJAsLO2enwjGWWueOO6BodiBgyAOZ/5t5nJNMCNuLGT5UIo/RI1b0WRQwxEZTRjt6mFNw6lH14wRd8ulsr9toSWBPMOGWoYs1PDeDL0JuTjL+tr1SZi/EyxCngpYszKdXllJEHyI79KQgeD0Vt3pTrkbNVTOEcCNqZePSVmUH8X8Vhugz3bnE0/iE9Pb5fkWO9c4AnM1FgI/8Bvp27Fw2ShryIXuR6kKvUqhVMTuOSDHwu6A8jLE5Owt3GAYugDpDYuwTVNGrHLXKpPzrGGPE/jPmaLCMZcsdkec95dYeU3zKODEm8UQZFhmJmDeWVJ36nGrGZHL4J5aTTaeFUJmmXDaJYiJ+K2/ioKgXqnXvltu0A9R8/LGy4nrTJRr4JMLuJFoUXvGm1gXQ70w2LSpk6yl71RNC0hCtsBe8BP8IhYCM0EP5jh7eCMQZNvM= nocomment
# comment asmdna,ndp
ssh-dss AAAAB3NzaC1kc3MAAACBAOChCC7lf6Uo9n7BmZ6M8St19PZf4Tn59NriyboW2x/DZuYAz3ibZ2OkQ3S0SqDIa0HXSEJ1zaExQdmbO+Ux/wsytWZmCczWOVsaszBZSl90q8UnWlSH6P+/YA+RWJm5SFtuV9PtGIhyZgoNuz5kBQ7K139wuQsecdKktISwTakzAAAAFQCzKsO2JhNKlL+wwwLGOcLffoAmkwAAAIBpK7/3xvduajLBD/9vASqBQIHrgK2J+wiQnIb/Wzy0UsVmvfn8A+udRbBo+csM8xrSnlnlJnjkJS3qiM5g+eTwsLIV1IdKPEwmwB+VcP53Cw6lSyWyJcvhFb0N6s08NZysLzvj0N+ZC/FnhKTLzIyMtkHf/IrPCwlM+pV/M/96YgAAAIEAqQcGn9CKgzgPaguIZooTAOQdvBLMI5y0bQjOW6734XOpqQGf/Kra90wpoasLKZjSYKNPjE+FRUOrStLrxcNs4BeVKhy2PYTRnybfYVk1/dmKgH6P1YSRONsGKvTsH6c5IyCRG0ncCgYeF8tXppyd642982daopE7zQ/NPAnJfag= nocomment`,
			number: 2,
			keyContents: []string{
				"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC4cn+iXnA4KvcQYSV88vGn0Yi91vG47t1P7okprVmhNTkipNRIHWr6WdCO4VDr/cvsRkuVJAsLO2enwjGWWueOO6BodiBgyAOZ/5t5nJNMCNuLGT5UIo/RI1b0WRQwxEZTRjt6mFNw6lH14wRd8ulsr9toSWBPMOGWoYs1PDeDL0JuTjL+tr1SZi/EyxCngpYszKdXllJEHyI79KQgeD0Vt3pTrkbNVTOEcCNqZePSVmUH8X8Vhugz3bnE0/iE9Pb5fkWO9c4AnM1FgI/8Bvp27Fw2ShryIXuR6kKvUqhVMTuOSDHwu6A8jLE5Owt3GAYugDpDYuwTVNGrHLXKpPzrGGPE/jPmaLCMZcsdkec95dYeU3zKODEm8UQZFhmJmDeWVJ36nGrGZHL4J5aTTaeFUJmmXDaJYiJ+K2/ioKgXqnXvltu0A9R8/LGy4nrTJRr4JMLuJFoUXvGm1gXQ70w2LSpk6yl71RNC0hCtsBe8BP8IhYCM0EP5jh7eCMQZNvM=",
				"ssh-dss AAAAB3NzaC1kc3MAAACBAOChCC7lf6Uo9n7BmZ6M8St19PZf4Tn59NriyboW2x/DZuYAz3ibZ2OkQ3S0SqDIa0HXSEJ1zaExQdmbO+Ux/wsytWZmCczWOVsaszBZSl90q8UnWlSH6P+/YA+RWJm5SFtuV9PtGIhyZgoNuz5kBQ7K139wuQsecdKktISwTakzAAAAFQCzKsO2JhNKlL+wwwLGOcLffoAmkwAAAIBpK7/3xvduajLBD/9vASqBQIHrgK2J+wiQnIb/Wzy0UsVmvfn8A+udRbBo+csM8xrSnlnlJnjkJS3qiM5g+eTwsLIV1IdKPEwmwB+VcP53Cw6lSyWyJcvhFb0N6s08NZysLzvj0N+ZC/FnhKTLzIyMtkHf/IrPCwlM+pV/M/96YgAAAIEAqQcGn9CKgzgPaguIZooTAOQdvBLMI5y0bQjOW6734XOpqQGf/Kra90wpoasLKZjSYKNPjE+FRUOrStLrxcNs4BeVKhy2PYTRnybfYVk1/dmKgH6P1YSRONsGKvTsH6c5IyCRG0ncCgYeF8tXppyd642982daopE7zQ/NPAnJfag=",
			},
		},
		{
			keyString: `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC4cn+iXnA4KvcQYSV88vGn0Yi91vG47t1P7okprVmhNTkipNRIHWr6WdCO4VDr/cvsRkuVJAsLO2enwjGWWueOO6BodiBgyAOZ/5t5nJNMCNuLGT5UIo/RI1b0WRQwxEZTRjt6mFNw6lH14wRd8ulsr9toSWBPMOGWoYs1PDeDL0JuTjL+tr1SZi/EyxCngpYszKdXllJEHyI79KQgeD0Vt3pTrkbNVTOEcCNqZePSVmUH8X8Vhugz3bnE0/iE9Pb5fkWO9c4AnM1FgI/8Bvp27Fw2ShryIXuR6kKvUqhVMTuOSDHwu6A8jLE5Owt3GAYugDpDYuwTVNGrHLXKpPzrGGPE/jPmaLCMZcsdkec95dYeU3zKODEm8UQZFhmJmDeWVJ36nGrGZHL4J5aTTaeFUJmmXDaJYiJ+K2/ioKgXqnXvltu0A9R8/LGy4nrTJRr4JMLuJFoUXvGm1gXQ70w2LSpk6yl71RNC0hCtsBe8BP8IhYCM0EP5jh7eCMQZNvM= nocomment
382488320jasdj1lasmva/vasodifipi4193-fksma.cm
ssh-dss AAAAB3NzaC1kc3MAAACBAOChCC7lf6Uo9n7BmZ6M8St19PZf4Tn59NriyboW2x/DZuYAz3ibZ2OkQ3S0SqDIa0HXSEJ1zaExQdmbO+Ux/wsytWZmCczWOVsaszBZSl90q8UnWlSH6P+/YA+RWJm5SFtuV9PtGIhyZgoNuz5kBQ7K139wuQsecdKktISwTakzAAAAFQCzKsO2JhNKlL+wwwLGOcLffoAmkwAAAIBpK7/3xvduajLBD/9vASqBQIHrgK2J+wiQnIb/Wzy0UsVmvfn8A+udRbBo+csM8xrSnlnlJnjkJS3qiM5g+eTwsLIV1IdKPEwmwB+VcP53Cw6lSyWyJcvhFb0N6s08NZysLzvj0N+ZC/FnhKTLzIyMtkHf/IrPCwlM+pV/M/96YgAAAIEAqQcGn9CKgzgPaguIZooTAOQdvBLMI5y0bQjOW6734XOpqQGf/Kra90wpoasLKZjSYKNPjE+FRUOrStLrxcNs4BeVKhy2PYTRnybfYVk1/dmKgH6P1YSRONsGKvTsH6c5IyCRG0ncCgYeF8tXppyd642982daopE7zQ/NPAnJfag= nocomment`,
			number: 2,
			keyContents: []string{
				"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC4cn+iXnA4KvcQYSV88vGn0Yi91vG47t1P7okprVmhNTkipNRIHWr6WdCO4VDr/cvsRkuVJAsLO2enwjGWWueOO6BodiBgyAOZ/5t5nJNMCNuLGT5UIo/RI1b0WRQwxEZTRjt6mFNw6lH14wRd8ulsr9toSWBPMOGWoYs1PDeDL0JuTjL+tr1SZi/EyxCngpYszKdXllJEHyI79KQgeD0Vt3pTrkbNVTOEcCNqZePSVmUH8X8Vhugz3bnE0/iE9Pb5fkWO9c4AnM1FgI/8Bvp27Fw2ShryIXuR6kKvUqhVMTuOSDHwu6A8jLE5Owt3GAYugDpDYuwTVNGrHLXKpPzrGGPE/jPmaLCMZcsdkec95dYeU3zKODEm8UQZFhmJmDeWVJ36nGrGZHL4J5aTTaeFUJmmXDaJYiJ+K2/ioKgXqnXvltu0A9R8/LGy4nrTJRr4JMLuJFoUXvGm1gXQ70w2LSpk6yl71RNC0hCtsBe8BP8IhYCM0EP5jh7eCMQZNvM=",
				"ssh-dss AAAAB3NzaC1kc3MAAACBAOChCC7lf6Uo9n7BmZ6M8St19PZf4Tn59NriyboW2x/DZuYAz3ibZ2OkQ3S0SqDIa0HXSEJ1zaExQdmbO+Ux/wsytWZmCczWOVsaszBZSl90q8UnWlSH6P+/YA+RWJm5SFtuV9PtGIhyZgoNuz5kBQ7K139wuQsecdKktISwTakzAAAAFQCzKsO2JhNKlL+wwwLGOcLffoAmkwAAAIBpK7/3xvduajLBD/9vASqBQIHrgK2J+wiQnIb/Wzy0UsVmvfn8A+udRbBo+csM8xrSnlnlJnjkJS3qiM5g+eTwsLIV1IdKPEwmwB+VcP53Cw6lSyWyJcvhFb0N6s08NZysLzvj0N+ZC/FnhKTLzIyMtkHf/IrPCwlM+pV/M/96YgAAAIEAqQcGn9CKgzgPaguIZooTAOQdvBLMI5y0bQjOW6734XOpqQGf/Kra90wpoasLKZjSYKNPjE+FRUOrStLrxcNs4BeVKhy2PYTRnybfYVk1/dmKgH6P1YSRONsGKvTsH6c5IyCRG0ncCgYeF8tXppyd642982daopE7zQ/NPAnJfag=",
			},
		},
	}

	for i, kase := range testCases {
		s.ID = int64(i) + 20
		AddPublicKeysBySource(user, s, []string{kase.keyString})
		keys, err := ListPublicKeysBySource(user.ID, s.ID)
		assert.NoError(t, err)
		if err != nil {
			continue
		}
		assert.Len(t, keys, kase.number)

		for _, key := range keys {
			assert.Contains(t, kase.keyContents, key.Content)
		}
		for _, key := range keys {
			DeletePublicKey(user, key.ID)
		}
	}
}

func TestUpdateUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)

	user.KeepActivityPrivate = true
	assert.NoError(t, UpdateUser(user))
	user = unittest.AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	assert.True(t, user.KeepActivityPrivate)

	setting.Service.AllowedUserVisibilityModesSlice = []bool{true, false, false}
	user.KeepActivityPrivate = false
	user.Visibility = structs.VisibleTypePrivate
	assert.Error(t, UpdateUser(user))
	user = unittest.AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	assert.True(t, user.KeepActivityPrivate)

	user.Email = "no mail@mail.org"
	assert.Error(t, UpdateUser(user))
}

func TestNewUserRedirect(t *testing.T) {
	// redirect to a completely new name
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &User{ID: 1}).(*User)
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

	user := unittest.AssertExistsAndLoadBean(t, &User{ID: 1}).(*User)
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

	user := unittest.AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	assert.NoError(t, user_model.NewUserRedirect(db.DefaultContext, user.ID, user.Name, "newusername"))

	unittest.AssertExistsAndLoadBean(t, &user_model.Redirect{
		LowerName:      user.LowerName,
		RedirectUserID: user.ID,
	})
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

	unittest.CheckConsistencyFor(t, &User{})
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

	unittest.CheckConsistencyFor(t, &User{})
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
