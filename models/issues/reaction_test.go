// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	issues_model "gitea.dev/models/issues"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestReactionListGetFirstUsersUsesConfiguredDisplayName(t *testing.T) {
	defer test.MockVariableValue(&setting.UI.ReactionMaxUserNum, 2)()

	list := issues_model.ReactionList{
		{User: &user_model.User{Name: "user1", FullName: "User One"}},
		{User: &user_model.User{Name: "user2", FullName: "User Two"}},
	}

	t.Run("full names enabled", func(t *testing.T) {
		defer test.MockVariableValue(&setting.UI.DefaultShowFullName, true)()
		assert.Equal(t, "User One, User Two", list.GetFirstUsers())
	})

	t.Run("full names disabled", func(t *testing.T) {
		defer test.MockVariableValue(&setting.UI.DefaultShowFullName, false)()
		assert.Equal(t, "user1, user2", list.GetFirstUsers())
	})
}
