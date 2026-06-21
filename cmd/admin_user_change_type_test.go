// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"io"
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChangeTypeCommand(t *testing.T) {
	ctx := t.Context()

	defer func() {
		require.NoError(t, db.TruncateBeans(t.Context(), &user_model.User{}))
		require.NoError(t, db.TruncateBeans(t.Context(), &user_model.EmailAddress{}))
	}()

	t.Run("convert individual to bot and back", func(t *testing.T) {
		require.NoError(t, microcmdUserCreate().Run(ctx, []string{"create", "--username", "testuser", "--email", "testuser@gitea.local", "--random-password"}))
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "testuser"})
		assert.True(t, user.IsIndividual())

		require.NoError(t, microcmdUserChangeType().Run(ctx, []string{"change-type", "--username", "testuser", "--user-type", "bot"}))
		user = unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "testuser"})
		assert.True(t, user.IsTypeBot())
		assert.Empty(t, user.Passwd)

		require.NoError(t, microcmdUserChangeType().Run(ctx, []string{"change-type", "--username", "testuser", "--user-type", "individual"}))
		user = unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "testuser"})
		assert.True(t, user.IsIndividual())
	})

	t.Run("failure cases", func(t *testing.T) {
		testCases := []struct {
			name        string
			args        []string
			expectedErr string
		}{
			{
				name:        "invalid user type",
				args:        []string{"change-type", "--username", "testuser", "--user-type", "invalid"},
				expectedErr: "invalid user type",
			},
			{
				name:        "missing username",
				args:        []string{"change-type", "--user-type", "bot"},
				expectedErr: `"username" not set`,
			},
			{
				name:        "missing user-type",
				args:        []string{"change-type", "--username", "testuser"},
				expectedErr: `"user-type" not set`,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				cmd := microcmdUserChangeType()
				cmd.Writer, cmd.ErrWriter = io.Discard, io.Discard
				err := cmd.Run(ctx, tc.args)
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErr)
			})
		}
	})
}
