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
)

func TestAdminUserCreate(t *testing.T) {
	app := NewMainApp(AppVersion{})

	reset := func() {
		assert.NoError(t, db.TruncateBeans(db.DefaultContext, &user_model.User{}))
		assert.NoError(t, db.TruncateBeans(db.DefaultContext, &user_model.EmailAddress{}))
	}

	type createCheck struct{ IsAdmin, MustChangePassword bool }
	createUser := func(name, args string) createCheck {
		assert.NoError(t, app.Run(strings.Fields(fmt.Sprintf("./gitea admin user create --username %s --email %s@gitea.local %s --password foobar", name, name, args))))
		u := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: name})
		return createCheck{u.IsAdmin, u.MustChangePassword}
	}
	reset()
	assert.Equal(t, createCheck{IsAdmin: false, MustChangePassword: false}, createUser("u", ""), "first non-admin user doesn't need to change password")

	reset()
	assert.Equal(t, createCheck{IsAdmin: true, MustChangePassword: false}, createUser("u", "--admin"), "first admin user doesn't need to change password")

	reset()
	assert.Equal(t, createCheck{IsAdmin: true, MustChangePassword: true}, createUser("u", "--admin --must-change-password"))
	assert.Equal(t, createCheck{IsAdmin: true, MustChangePassword: true}, createUser("u2", "--admin"))
	assert.Equal(t, createCheck{IsAdmin: true, MustChangePassword: false}, createUser("u3", "--admin --must-change-password=false"))
	assert.Equal(t, createCheck{IsAdmin: false, MustChangePassword: true}, createUser("u4", ""))
	assert.Equal(t, createCheck{IsAdmin: false, MustChangePassword: false}, createUser("u5", "--must-change-password=false"))
}
