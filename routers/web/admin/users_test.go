// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"testing"

	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/web"
	"gitea.dev/services/contexttest"
	"gitea.dev/services/forms"

	"github.com/stretchr/testify/assert"
)

func TestNewUserPost_MustChangePassword(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "admin/users/new")

	u := unittest.AssertExistsAndLoadBean(t, &user_model.User{
		IsAdmin: true,
		ID:      2,
	})

	ctx.Doer = u

	username := "gitea"
	email := "gitea@gitea.io"

	form := forms.AdminCreateUserForm{
		LoginType:          "local",
		LoginName:          "local",
		UserName:           username,
		Email:              email,
		Password:           "abc123ABC!=$",
		SendNotify:         false,
		MustChangePassword: true,
	}

	web.SetForm(ctx, &form)
	NewUserPost(ctx)

	assert.NotEmpty(t, ctx.Flash.SuccessMsg)

	u, err := user_model.GetUserByName(ctx, username)

	assert.NoError(t, err)
	assert.Equal(t, username, u.Name)
	assert.Equal(t, email, u.Email)
	assert.True(t, u.MustChangePassword)
}

func TestNewUserPost_MustChangePasswordFalse(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "admin/users/new")

	u := unittest.AssertExistsAndLoadBean(t, &user_model.User{
		IsAdmin: true,
		ID:      2,
	})

	ctx.Doer = u

	username := "gitea"
	email := "gitea@gitea.io"

	form := forms.AdminCreateUserForm{
		LoginType:          "local",
		LoginName:          "local",
		UserName:           username,
		Email:              email,
		Password:           "abc123ABC!=$",
		SendNotify:         false,
		MustChangePassword: false,
	}

	web.SetForm(ctx, &form)
	NewUserPost(ctx)

	assert.NotEmpty(t, ctx.Flash.SuccessMsg)

	u, err := user_model.GetUserByName(ctx, username)

	assert.NoError(t, err)
	assert.Equal(t, username, u.Name)
	assert.Equal(t, email, u.Email)
	assert.False(t, u.MustChangePassword)
}

func TestNewUserPost_Bot(t *testing.T) {
	unittest.PrepareTestEnv(t)

	t.Run("rejects password", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "admin/users/new")
		ctx.Doer = unittest.AssertExistsAndLoadBean(t, &user_model.User{IsAdmin: true, ID: 2})

		web.SetForm(ctx, &forms.AdminCreateUserForm{
			LoginType: "local",
			UserName:  "bot-with-pw",
			UserType:  "bot",
			Email:     "bot-with-pw@gitea.io",
			Password:  "abc123ABC!=$",
		})
		NewUserPost(ctx)

		// a bot must not be created with a password
		assert.NotEmpty(t, ctx.Flash.ErrorMsg)
		unittest.AssertNotExistsBean(t, &user_model.User{LowerName: "bot-with-pw"})
	})

	t.Run("creates passwordless bot", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "admin/users/new")
		ctx.Doer = unittest.AssertExistsAndLoadBean(t, &user_model.User{IsAdmin: true, ID: 2})

		web.SetForm(ctx, &forms.AdminCreateUserForm{
			LoginType: "local",
			UserName:  "bot-user",
			UserType:  "bot",
			Email:     "bot-user@gitea.io",
		})
		NewUserPost(ctx)

		assert.NotEmpty(t, ctx.Flash.SuccessMsg)
		u, err := user_model.GetUserByName(ctx, "bot-user")
		assert.NoError(t, err)
		assert.True(t, u.IsTypeBot())
		assert.Empty(t, u.Passwd)
		assert.Empty(t, u.Salt)
		assert.False(t, u.MustChangePassword)
	})
}

func TestConvertUserType(t *testing.T) {
	unittest.PrepareTestEnv(t)

	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{IsAdmin: true, ID: 2})

	t.Run("to bot", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "admin/users/4/convert_type?user_type=bot")
		ctx.Doer = doer
		ctx.SetPathParam("userid", "4")
		ConvertUserType(ctx)

		assert.NotEmpty(t, ctx.Flash.SuccessMsg)
		u := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
		assert.True(t, u.IsTypeBot())
		assert.Empty(t, u.Passwd)
	})

	t.Run("back to individual", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "admin/users/4/convert_type?user_type=individual")
		ctx.Doer = doer
		ctx.SetPathParam("userid", "4")
		ConvertUserType(ctx)

		assert.NotEmpty(t, ctx.Flash.SuccessMsg)
		u := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
		assert.True(t, u.IsIndividual())
	})

	t.Run("invalid user type", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "admin/users/4/convert_type?user_type=invalid")
		ctx.Doer = doer
		ctx.SetPathParam("userid", "4")
		ConvertUserType(ctx)

		assert.NotEmpty(t, ctx.Flash.ErrorMsg)
		u := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
		assert.True(t, u.IsIndividual())
	})
}

func TestNewUserPost_InvalidEmail(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "admin/users/new")

	u := unittest.AssertExistsAndLoadBean(t, &user_model.User{
		IsAdmin: true,
		ID:      2,
	})

	ctx.Doer = u

	username := "gitea"
	email := "gitea@gitea.io\r\n"

	form := forms.AdminCreateUserForm{
		LoginType:          "local",
		LoginName:          "local",
		UserName:           username,
		Email:              email,
		Password:           "abc123ABC!=$",
		SendNotify:         false,
		MustChangePassword: false,
	}

	web.SetForm(ctx, &form)
	NewUserPost(ctx)

	assert.NotEmpty(t, ctx.Flash.ErrorMsg)
}

func TestNewUserPost_VisibilityDefaultPublic(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "admin/users/new")

	u := unittest.AssertExistsAndLoadBean(t, &user_model.User{
		IsAdmin: true,
		ID:      2,
	})

	ctx.Doer = u

	username := "gitea"
	email := "gitea@gitea.io"

	form := forms.AdminCreateUserForm{
		LoginType:          "local",
		LoginName:          "local",
		UserName:           username,
		Email:              email,
		Password:           "abc123ABC!=$",
		SendNotify:         false,
		MustChangePassword: false,
	}

	web.SetForm(ctx, &form)
	NewUserPost(ctx)

	assert.NotEmpty(t, ctx.Flash.SuccessMsg)

	u, err := user_model.GetUserByName(ctx, username)

	assert.NoError(t, err)
	assert.Equal(t, username, u.Name)
	assert.Equal(t, email, u.Email)
	// As default user visibility
	assert.Equal(t, setting.Service.DefaultUserVisibilityMode, u.Visibility)
}

func TestNewUserPost_VisibilityPrivate(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "admin/users/new")

	u := unittest.AssertExistsAndLoadBean(t, &user_model.User{
		IsAdmin: true,
		ID:      2,
	})

	ctx.Doer = u

	username := "gitea"
	email := "gitea@gitea.io"

	form := forms.AdminCreateUserForm{
		LoginType:          "local",
		LoginName:          "local",
		UserName:           username,
		Email:              email,
		Password:           "abc123ABC!=$",
		SendNotify:         false,
		MustChangePassword: false,
		Visibility:         api.VisibleTypePrivate,
	}

	web.SetForm(ctx, &form)
	NewUserPost(ctx)

	assert.NotEmpty(t, ctx.Flash.SuccessMsg)

	u, err := user_model.GetUserByName(ctx, username)

	assert.NoError(t, err)
	assert.Equal(t, username, u.Name)
	assert.Equal(t, email, u.Email)
	// As default user visibility
	assert.True(t, u.Visibility.IsPrivate())
}
