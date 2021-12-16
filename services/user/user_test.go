// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, filepath.Join("..", ".."))
}

func TestDeleteUser(t *testing.T) {
	test := func(userID int64) {
		assert.NoError(t, unittest.PrepareTestDatabase())
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: userID}).(*user_model.User)

		ownedRepos := make([]*repo_model.Repository, 0, 10)
		assert.NoError(t, db.GetEngine(db.DefaultContext).Find(&ownedRepos, &repo_model.Repository{OwnerID: userID}))
		if len(ownedRepos) > 0 {
			err := DeleteUser(user)
			assert.Error(t, err)
			assert.True(t, models.IsErrUserOwnRepos(err))
			return
		}

		orgUsers := make([]*models.OrgUser, 0, 10)
		assert.NoError(t, db.GetEngine(db.DefaultContext).Find(&orgUsers, &models.OrgUser{UID: userID}))
		for _, orgUser := range orgUsers {
			if err := models.RemoveOrgUser(orgUser.OrgID, orgUser.UID); err != nil {
				assert.True(t, models.IsErrLastOrgOwner(err))
				return
			}
		}
		assert.NoError(t, DeleteUser(user))
		unittest.AssertNotExistsBean(t, &user_model.User{ID: userID})
		unittest.CheckConsistencyFor(t, &user_model.User{}, &repo_model.Repository{})
	}
	test(2)
	test(4)
	test(8)
	test(11)

	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3}).(*user_model.User)
	assert.Error(t, DeleteUser(org))
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

	assert.NoError(t, user_model.CreateUser(user))

	assert.NoError(t, DeleteUser(user))
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

		assert.NoError(t, user_model.CreateUser(v.user))

		u, err := user_model.GetUserByEmail(v.user.Email)
		assert.NoError(t, err)

		assert.Equal(t, !u.AllowCreateOrganization, v.disableOrgCreation)

		assert.NoError(t, DeleteUser(v.user))
	}
}
