// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"strconv"
	"strings"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/require"
)

func TestAdminUserDelete(t *testing.T) {
	ctx := t.Context()
	defer func() {
		require.NoError(t, db.TruncateBeans(t.Context(), &user_model.User{}))
		require.NoError(t, db.TruncateBeans(t.Context(), &user_model.EmailAddress{}))
		require.NoError(t, db.TruncateBeans(t.Context(), &auth_model.AccessToken{}))
	}()

	setupTestUser := func(t *testing.T) {
		unittest.AssertNotExistsBean(t, &user_model.User{LowerName: "testuser"})
		err := microcmdUserCreate().Run(t.Context(), []string{"create", "--username", "testuser", "--email", "testuser@gitea.local", "--random-password"})
		require.NoError(t, err)
	}

	t.Run("delete user by id", func(t *testing.T) {
		setupTestUser(t)

		u := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "testuser"})
		err := microcmdUserDelete().Run(ctx, []string{"delete-test", "--id", strconv.FormatInt(u.ID, 10)})
		require.NoError(t, err)
		unittest.AssertNotExistsBean(t, &user_model.User{LowerName: "testuser"})
	})
	t.Run("delete user by username", func(t *testing.T) {
		setupTestUser(t)

		err := microcmdUserDelete().Run(ctx, []string{"delete-test", "--username", "testuser"})
		require.NoError(t, err)
		unittest.AssertNotExistsBean(t, &user_model.User{LowerName: "testuser"})
	})
	t.Run("delete user by email", func(t *testing.T) {
		setupTestUser(t)

		err := microcmdUserDelete().Run(ctx, []string{"delete-test", "--email", "testuser@gitea.local"})
		require.NoError(t, err)
		unittest.AssertNotExistsBean(t, &user_model.User{LowerName: "testuser"})
	})
	t.Run("delete user by all 3 attributes", func(t *testing.T) {
		setupTestUser(t)

		u := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "testuser"})
		err := microcmdUserDelete().Run(ctx, []string{"delete", "--id", strconv.FormatInt(u.ID, 10), "--username", "testuser", "--email", "testuser@gitea.local"})
		require.NoError(t, err)
		unittest.AssertNotExistsBean(t, &user_model.User{LowerName: "testuser"})
	})
}

func TestAdminUserDeleteFailure(t *testing.T) {
	testCases := []struct {
		name        string
		args        []string
		expectedErr string
	}{
		{
			name:        "no user to delete",
			args:        []string{"delete", "--username", "nonexistentuser"},
			expectedErr: "user does not exist",
		},
		{
			name:        "user exists but provided username does not match",
			args:        []string{"delete", "--email", "testuser@gitea.local", "--username", "wrongusername"},
			expectedErr: "the user testuser who has email testuser@gitea.local does not match the provided username wrongusername",
		},
		{
			name:        "user exists but provided id does not match",
			args:        []string{"delete", "--username", "testuser", "--id", "999"},
			expectedErr: "the user testuser does not match the provided id 999",
		},
		{
			name:        "no required flags are provided",
			args:        []string{"delete"},
			expectedErr: "You must provide the id, username or email of a user to delete",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			if strings.Contains(tc.name, "user exists") {
				unittest.AssertNotExistsBean(t, &user_model.User{LowerName: "testuser"})
				err := microcmdUserCreate().Run(t.Context(), []string{"create", "--username", "testuser", "--email", "testuser@gitea.local", "--random-password"})
				require.NoError(t, err)
			}

			err := microcmdUserDelete().Run(ctx, tc.args)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expectedErr)
		})

		require.NoError(t, db.TruncateBeans(t.Context(), &user_model.User{}))
		require.NoError(t, db.TruncateBeans(t.Context(), &user_model.EmailAddress{}))
		require.NoError(t, db.TruncateBeans(t.Context(), &auth_model.AccessToken{}))
	}
}
