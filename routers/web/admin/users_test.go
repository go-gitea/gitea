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

func TestNewUserPost(t *testing.T) {
	for _, tc := range []struct {
		name               string
		email              string
		mustChangePassword bool
		visibility         api.VisibleType
	}{
		{name: "MustChangePassword", email: "gitea@gitea.io", mustChangePassword: true},
		{name: "MustChangePasswordFalseAndDefaultVisibility", email: "gitea@gitea.io"},
		{name: "InvalidEmail", email: "gitea@gitea.io\r\n"},
		{name: "VisibilityPrivate", email: "gitea@gitea.io", visibility: api.VisibleTypePrivate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.NoError(t, unittest.PrepareTestDatabase())
			ctx, _ := contexttest.MockContext(t, "admin/users/new")
			ctx.Doer = unittest.AssertExistsAndLoadBean(t, &user_model.User{IsAdmin: true, ID: 2})

			web.SetForm(ctx, &forms.AdminCreateUserForm{
				LoginType:          "local",
				LoginName:          "local",
				UserName:           "gitea",
				Email:              tc.email,
				Password:           "abc123ABC!=$",
				SendNotify:         false,
				MustChangePassword: tc.mustChangePassword,
				Visibility:         tc.visibility,
			})
			NewUserPost(ctx)

			if tc.name == "InvalidEmail" {
				assert.NotEmpty(t, ctx.Flash.ErrorMsg)
				return
			}
			assert.NotEmpty(t, ctx.Flash.SuccessMsg)
			user, err := user_model.GetUserByName(ctx, "gitea")
			assert.NoError(t, err)
			assert.Equal(t, "gitea", user.Name)
			assert.Equal(t, tc.email, user.Email)
			switch tc.name {
			case "MustChangePassword":
				assert.True(t, user.MustChangePassword)
			case "MustChangePasswordFalseAndDefaultVisibility":
				assert.False(t, user.MustChangePassword)
				// As default user visibility
				assert.Equal(t, setting.Service.DefaultUserVisibilityMode, user.Visibility)
			case "VisibilityPrivate":
				// As default user visibility
				assert.True(t, user.Visibility.IsPrivate())
			}
		})
	}
}
