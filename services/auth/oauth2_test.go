// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"fmt"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/services/actions"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserIDFromToken(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	t.Run("Actions JWT", func(t *testing.T) {
		const RunningTaskID int64 = 47
		token, err := actions.CreateAuthorizationToken(RunningTaskID, 1, 2)
		assert.NoError(t, err)

		o := OAuth2{}
		u, err := o.userFromToken(t.Context(), token)
		require.NoError(t, err)
		assert.Equal(t, user_model.ActionsUserID, u.ID)
		taskID, ok := user_model.GetActionsUserTaskID(u)
		assert.True(t, ok)
		assert.Equal(t, RunningTaskID, taskID)
	})

	t.Run("Access token", func(t *testing.T) {
		token := &auth_model.AccessToken{UID: 2, Name: "public-only", Scope: "public-only,read:repository"}
		require.NoError(t, auth_model.NewAccessToken(t.Context(), token))

		u, err := (&OAuth2{}).userFromToken(t.Context(), token.Token)
		require.NoError(t, err)
		scope, _ := user_model.GetDoerTokenScope(u)
		assert.Equal(t, token.Scope, scope)
		assert.Equal(t, fmt.Sprintf("access-token:%d", token.ID), user_model.GetDoerCredential(u))
	})
}

func TestCheckTaskIsRunning(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	cases := map[string]struct {
		TaskID   int64
		Expected bool
	}{
		"Running":   {TaskID: 47, Expected: true},
		"Missing":   {TaskID: 1, Expected: false},
		"Cancelled": {TaskID: 46, Expected: false},
	}

	for name := range cases {
		c := cases[name]
		t.Run(name, func(t *testing.T) {
			actual := CheckTaskIsRunning(t.Context(), c.TaskID)
			assert.Equal(t, c.Expected, actual)
		})
	}
}
