// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"fmt"
	"strings"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestDeleteUser(t *testing.T) {
	test := func(userID int64) {
		assert.NoError(t, unittest.PrepareTestDatabase())
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: userID})

		ownedRepos := make([]*repo_model.Repository, 0, 10)
		assert.NoError(t, db.GetEngine(db.DefaultContext).Find(&ownedRepos, &repo_model.Repository{OwnerID: userID}))
		if len(ownedRepos) > 0 {
			err := DeleteUser(db.DefaultContext, user, false)
			assert.Error(t, err)
			assert.True(t, models.IsErrUserOwnRepos(err))
			return
		}

		orgUsers := make([]*organization.OrgUser, 0, 10)
		assert.NoError(t, db.GetEngine(db.DefaultContext).Find(&orgUsers, &organization.OrgUser{UID: userID}))
		for _, orgUser := range orgUsers {
			if err := models.RemoveOrgUser(db.DefaultContext, orgUser.OrgID, orgUser.UID); err != nil {
				assert.True(t, organization.IsErrLastOrgOwner(err))
				return
			}
		}
		assert.NoError(t, DeleteUser(db.DefaultContext, user, false))
		unittest.AssertNotExistsBean(t, &user_model.User{ID: userID})
		unittest.CheckConsistencyFor(t, &user_model.User{}, &repo_model.Repository{})
	}
	test(2)
	test(4)
	test(8)
	test(11)

	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
	assert.Error(t, DeleteUser(db.DefaultContext, org, false))
}

func TestPurgeUser(t *testing.T) {
	test := func(userID int64) {
		assert.NoError(t, unittest.PrepareTestDatabase())
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: userID})

		err := DeleteUser(db.DefaultContext, user, true)
		assert.NoError(t, err)

		unittest.AssertNotExistsBean(t, &user_model.User{ID: userID})
		unittest.CheckConsistencyFor(t, &user_model.User{}, &repo_model.Repository{})
	}
	test(2)
	test(4)
	test(8)
	test(11)

	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
	assert.Error(t, DeleteUser(db.DefaultContext, org, false))
}

func TestCreateUser(t *testing.T) {
	user := &user_model.User{
		Name:               "GiteaBot",
		Email:              "GiteaBot@gitea.io",
		Passwd:             ";p['////..-++']",
		IsAdmin:            false,
		Theme:              setting.UI.DefaultTheme,
		MustChangePassword: false,
	}

	assert.NoError(t, user_model.CreateUser(db.DefaultContext, user))

	assert.NoError(t, DeleteUser(db.DefaultContext, user, false))
}

func TestRenameUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 21})

	t.Run("Non-Local", func(t *testing.T) {
		u := &user_model.User{
			Type:      user_model.UserTypeIndividual,
			LoginType: auth.OAuth2,
		}
		assert.ErrorIs(t, RenameUser(db.DefaultContext, u, "user_rename"), user_model.ErrUserIsNotLocal{})
	})

	t.Run("Same username", func(t *testing.T) {
		assert.ErrorIs(t, RenameUser(db.DefaultContext, user, user.Name), user_model.ErrUsernameNotChanged{UID: user.ID, Name: user.Name})
	})

	t.Run("Non usable username", func(t *testing.T) {
		usernames := []string{"--diff", "aa.png", ".well-known", "search", "aaa.atom"}
		for _, username := range usernames {
			t.Run(username, func(t *testing.T) {
				assert.Error(t, user_model.IsUsableUsername(username))
				assert.Error(t, RenameUser(db.DefaultContext, user, username))
			})
		}
	})

	t.Run("Only capitalization", func(t *testing.T) {
		caps := strings.ToUpper(user.Name)
		unittest.AssertNotExistsBean(t, &user_model.User{ID: user.ID, Name: caps})
		unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: user.ID, OwnerName: user.Name})

		assert.NoError(t, RenameUser(db.DefaultContext, user, caps))

		unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: user.ID, Name: caps})
		unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: user.ID, OwnerName: caps})
	})

	t.Run("Already exists", func(t *testing.T) {
		existUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

		assert.ErrorIs(t, RenameUser(db.DefaultContext, user, existUser.Name), user_model.ErrUserAlreadyExist{Name: existUser.Name})
		assert.ErrorIs(t, RenameUser(db.DefaultContext, user, existUser.LowerName), user_model.ErrUserAlreadyExist{Name: existUser.LowerName})
		newUsername := fmt.Sprintf("uSEr%d", existUser.ID)
		assert.ErrorIs(t, RenameUser(db.DefaultContext, user, newUsername), user_model.ErrUserAlreadyExist{Name: newUsername})
	})

	t.Run("Normal", func(t *testing.T) {
		oldUsername := user.Name
		newUsername := "User_Rename"

		assert.NoError(t, RenameUser(db.DefaultContext, user, newUsername))
		unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: user.ID, Name: newUsername, LowerName: strings.ToLower(newUsername)})

		redirectUID, err := user_model.LookupUserRedirect(db.DefaultContext, oldUsername)
		assert.NoError(t, err)
		assert.EqualValues(t, user.ID, redirectUID)

		unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: user.ID, OwnerName: user.Name})
	})
}

func TestCreateUser_Issue5882(t *testing.T) {
	// Init settings
	_ = setting.Admin

	passwd := ".//.;1;;//.,-=_"

	tt := []struct {
		user               *user_model.User
		disableOrgCreation bool
	}{
		{&user_model.User{Name: "GiteaBot", Email: "GiteaBot@gitea.io", Passwd: passwd, MustChangePassword: false}, false},
		{&user_model.User{Name: "GiteaBot2", Email: "GiteaBot2@gitea.io", Passwd: passwd, MustChangePassword: false}, true},
	}

	setting.Service.DefaultAllowCreateOrganization = true

	for _, v := range tt {
		setting.Admin.DisableRegularOrgCreation = v.disableOrgCreation

		assert.NoError(t, user_model.CreateUser(db.DefaultContext, v.user))

		u, err := user_model.GetUserByEmail(db.DefaultContext, v.user.Email)
		assert.NoError(t, err)

		assert.Equal(t, !u.AllowCreateOrganization, v.disableOrgCreation)

		assert.NoError(t, DeleteUser(db.DefaultContext, v.user, false))
	}
}
