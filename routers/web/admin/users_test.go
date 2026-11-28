// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/services/contexttest"

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
		{name: "InvalidEmail", email: "gitea@@gitea.io"},
		{name: "VisibilityPrivate", email: "gitea@gitea.io", visibility: api.VisibleTypePrivate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.NoError(t, unittest.PrepareTestDatabase())
			ctx, resp := contexttest.MockContext(t, "POST /-/admin/users/new")
			ctx.Doer = unittest.AssertExistsAndLoadBean(t, &user_model.User{IsAdmin: true, ID: 2})

			ctx.Req.Form = url.Values{
				"login_type":           {"local"},
				"login_name":           {"local"},
				"user_name":            {"gitea"},
				"email":                {tc.email},
				"password":             {"abc123ABC!=$"},
				"must_change_password": {strconv.FormatBool(tc.mustChangePassword)},
				"visibility":           {strconv.Itoa(int(tc.visibility))},
			}

			ctx.Req.PostForm = ctx.Req.Form
			ctx.Req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			NewUserPost(ctx)

			if tc.name == "InvalidEmail" {
				assert.Equal(t, http.StatusBadRequest, resp.Code)
				assert.Contains(t, resp.Body.String(), `"errorFields":["Email"]`)
				return
			}
			assert.Equal(t, http.StatusOK, resp.Code)
			assert.Contains(t, resp.Body.String(), `"redirect":`)
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
