// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	password_module "code.gitea.io/gitea/modules/auth/password"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestUpdateUser(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	assert.Error(t, UpdateUser(db.DefaultContext, admin, &UpdateOptions{
		IsAdmin: optional.Some(false),
	}))

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 28})

	opts := &UpdateOptions{
		KeepEmailPrivate:             optional.Some(false),
		FullName:                     optional.Some("Changed Name"),
		Website:                      optional.Some("https://gitea.com/"),
		Location:                     optional.Some("location"),
		Description:                  optional.Some("description"),
		AllowGitHook:                 optional.Some(true),
		AllowImportLocal:             optional.Some(true),
		MaxRepoCreation:              optional.Some(10),
		IsRestricted:                 optional.Some(true),
		IsActive:                     optional.Some(false),
		IsAdmin:                      optional.Some(true),
		Visibility:                   optional.Some(structs.VisibleTypePrivate),
		KeepActivityPrivate:          optional.Some(true),
		Language:                     optional.Some("lang"),
		Theme:                        optional.Some("theme"),
		DiffViewStyle:                optional.Some("split"),
		AllowCreateOrganization:      optional.Some(false),
		EmailNotificationsPreference: optional.Some("disabled"),
		SetLastLogin:                 true,
	}
	assert.NoError(t, UpdateUser(db.DefaultContext, user, opts))

	assert.Equal(t, opts.KeepEmailPrivate.Value(), user.KeepEmailPrivate)
	assert.Equal(t, opts.FullName.Value(), user.FullName)
	assert.Equal(t, opts.Website.Value(), user.Website)
	assert.Equal(t, opts.Location.Value(), user.Location)
	assert.Equal(t, opts.Description.Value(), user.Description)
	assert.Equal(t, opts.AllowGitHook.Value(), user.AllowGitHook)
	assert.Equal(t, opts.AllowImportLocal.Value(), user.AllowImportLocal)
	assert.Equal(t, opts.MaxRepoCreation.Value(), user.MaxRepoCreation)
	assert.Equal(t, opts.IsRestricted.Value(), user.IsRestricted)
	assert.Equal(t, opts.IsActive.Value(), user.IsActive)
	assert.Equal(t, opts.IsAdmin.Value(), user.IsAdmin)
	assert.Equal(t, opts.Visibility.Value(), user.Visibility)
	assert.Equal(t, opts.KeepActivityPrivate.Value(), user.KeepActivityPrivate)
	assert.Equal(t, opts.Language.Value(), user.Language)
	assert.Equal(t, opts.Theme.Value(), user.Theme)
	assert.Equal(t, opts.DiffViewStyle.Value(), user.DiffViewStyle)
	assert.Equal(t, opts.AllowCreateOrganization.Value(), user.AllowCreateOrganization)
	assert.Equal(t, opts.EmailNotificationsPreference.Value(), user.EmailNotificationsPreference)

	user = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 28})
	assert.Equal(t, opts.KeepEmailPrivate.Value(), user.KeepEmailPrivate)
	assert.Equal(t, opts.FullName.Value(), user.FullName)
	assert.Equal(t, opts.Website.Value(), user.Website)
	assert.Equal(t, opts.Location.Value(), user.Location)
	assert.Equal(t, opts.Description.Value(), user.Description)
	assert.Equal(t, opts.AllowGitHook.Value(), user.AllowGitHook)
	assert.Equal(t, opts.AllowImportLocal.Value(), user.AllowImportLocal)
	assert.Equal(t, opts.MaxRepoCreation.Value(), user.MaxRepoCreation)
	assert.Equal(t, opts.IsRestricted.Value(), user.IsRestricted)
	assert.Equal(t, opts.IsActive.Value(), user.IsActive)
	assert.Equal(t, opts.IsAdmin.Value(), user.IsAdmin)
	assert.Equal(t, opts.Visibility.Value(), user.Visibility)
	assert.Equal(t, opts.KeepActivityPrivate.Value(), user.KeepActivityPrivate)
	assert.Equal(t, opts.Language.Value(), user.Language)
	assert.Equal(t, opts.Theme.Value(), user.Theme)
	assert.Equal(t, opts.DiffViewStyle.Value(), user.DiffViewStyle)
	assert.Equal(t, opts.AllowCreateOrganization.Value(), user.AllowCreateOrganization)
	assert.Equal(t, opts.EmailNotificationsPreference.Value(), user.EmailNotificationsPreference)
}

func TestUpdateAuth(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 28})
	userCopy := *user

	assert.NoError(t, UpdateAuth(db.DefaultContext, user, &UpdateAuthOptions{
		LoginName: optional.Some("new-login"),
	}))
	assert.Equal(t, "new-login", user.LoginName)

	assert.NoError(t, UpdateAuth(db.DefaultContext, user, &UpdateAuthOptions{
		Password:           optional.Some("%$DRZUVB576tfzgu"),
		MustChangePassword: optional.Some(true),
	}))
	assert.True(t, user.MustChangePassword)
	assert.NotEqual(t, userCopy.Passwd, user.Passwd)
	assert.NotEqual(t, userCopy.Salt, user.Salt)

	assert.NoError(t, UpdateAuth(db.DefaultContext, user, &UpdateAuthOptions{
		ProhibitLogin: optional.Some(true),
	}))
	assert.True(t, user.ProhibitLogin)

	assert.ErrorIs(t, UpdateAuth(db.DefaultContext, user, &UpdateAuthOptions{
		Password: optional.Some("aaaa"),
	}), password_module.ErrMinLength)
}
