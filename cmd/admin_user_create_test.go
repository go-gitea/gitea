// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminUserCreate(t *testing.T) {
	app := NewMainApp(AppVersion{})

	reset := func() {
		require.NoError(t, db.TruncateBeans(db.DefaultContext, &user_model.User{}))
		require.NoError(t, db.TruncateBeans(db.DefaultContext, &user_model.EmailAddress{}))
	}

	t.Run("MustChangePassword", func(t *testing.T) {
		type check struct {
			IsAdmin            bool
			MustChangePassword bool
		}
		createCheck := func(name, args string) check {
			require.NoError(t, app.Run(strings.Fields(fmt.Sprintf("./gitea admin user create --username %s --email %s@gitea.local %s --password foobar", name, name, args))))
			u := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: name})
			return check{IsAdmin: u.IsAdmin, MustChangePassword: u.MustChangePassword}
		}
		reset()
		assert.Equal(t, check{IsAdmin: false, MustChangePassword: false}, createCheck("u", ""), "first non-admin user doesn't need to change password")

		reset()
		assert.Equal(t, check{IsAdmin: true, MustChangePassword: false}, createCheck("u", "--admin"), "first admin user doesn't need to change password")

		reset()
		assert.Equal(t, check{IsAdmin: true, MustChangePassword: true}, createCheck("u", "--admin --must-change-password"))
		assert.Equal(t, check{IsAdmin: true, MustChangePassword: true}, createCheck("u2", "--admin"))
		assert.Equal(t, check{IsAdmin: true, MustChangePassword: false}, createCheck("u3", "--admin --must-change-password=false"))
		assert.Equal(t, check{IsAdmin: false, MustChangePassword: true}, createCheck("u4", ""))
		assert.Equal(t, check{IsAdmin: false, MustChangePassword: false}, createCheck("u5", "--must-change-password=false"))
	})

	t.Run("UserType", func(t *testing.T) {
		createUser := func(name, args string) error {
			return app.Run(strings.Fields(fmt.Sprintf("./gitea admin user create --username %s --email %s@gitea.local %s", name, name, args)))
		}

		reset()
		assert.ErrorContains(t, createUser("u", "--user-type invalid"), "invalid user type")
		assert.ErrorContains(t, createUser("u", "--user-type bot --password 123"), "can only be set for individual users")
		assert.ErrorContains(t, createUser("u", "--user-type bot --must-change-password"), "can only be set for individual users")

		assert.NoError(t, createUser("u", "--user-type bot"))
		u := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "u"})
		assert.Equal(t, user_model.UserTypeBot, u.Type)
		assert.Equal(t, "", u.Passwd)
	})
}
