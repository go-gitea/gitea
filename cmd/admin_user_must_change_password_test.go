// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMustChangePassword(t *testing.T) {
	defer func() {
		require.NoError(t, db.TruncateBeans(t.Context(), &user_model.User{}))
	}()
	for _, name := range []string{"testuser", "testuserexclude"} {
		require.NoError(t, microcmdUserCreate().Run(t.Context(), []string{"create", "--username", name, "--email", name + "@gitea.local", "--random-password"}))
	}
	for _, tc := range []struct {
		args                           []string
		assertUser, assertExcludedUser assert.BoolAssertionFunc
	}{
		// Reset password change flag
		{[]string{"change-test", "--all", "--unset"}, assert.False, assert.False},
		// Make all users change password
		{[]string{"change-test", "--all"}, assert.True, assert.True},
		// Reset password change flag but exclude all tested users
		{[]string{"change-test", "--all", "--unset", "--exclude", "testuser,testuserexclude"}, assert.True, assert.True},
		// Reset password change flag by listing multiple users
		{[]string{"change-test", "--unset", "testuser", "testuserexclude"}, assert.False, assert.False},
		// Exclude a user from all user
		{[]string{"change-test", "--all", "--exclude", "testuserexclude"}, assert.True, assert.False},
		// Unset a flag for single user
		{[]string{"change-test", "--unset", "testuser"}, assert.False, assert.False},
	} {
		require.NoError(t, microcmdUserMustChangePassword().Run(t.Context(), tc.args))
		tc.assertUser(t, unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "testuser"}).MustChangePassword)
		tc.assertExcludedUser(t, unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "testuserexclude"}).MustChangePassword)
	}
}
