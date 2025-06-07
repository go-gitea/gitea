// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMustChangePassword(t *testing.T) {
	defer func() {
		require.NoError(t, db.TruncateBeans(db.DefaultContext, &user_model.User{}))
	}()
	err := microcmdUserCreate().Run(t.Context(), []string{"create", "--username", "testuser", "--email", "testuser@gitea.local", "--random-password"})
	require.NoError(t, err)
	err = microcmdUserCreate().Run(t.Context(), []string{"create", "--username", "testuserexclude", "--email", "testuserexclude@gitea.local", "--random-password"})
	require.NoError(t, err)
	// Reset password change flag
	err = microcmdUserMustChangePassword().Run(t.Context(), []string{"change-test", "--all", "--unset"})
	require.NoError(t, err)

	testUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "testuser"})
	assert.False(t, testUser.MustChangePassword)
	testUserExclude := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "testuserexclude"})
	assert.False(t, testUserExclude.MustChangePassword)

	// Make all users change password
	err = microcmdUserMustChangePassword().Run(t.Context(), []string{"change-test", "--all"})
	require.NoError(t, err)

	testUser = unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "testuser"})
	assert.True(t, testUser.MustChangePassword)
	testUserExclude = unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "testuserexclude"})
	assert.True(t, testUserExclude.MustChangePassword)

	// Reset password change flag but exclude all tested users
	err = microcmdUserMustChangePassword().Run(t.Context(), []string{"change-test", "--all", "--unset", "--exclude", "testuser,testuserexclude"})
	require.NoError(t, err)

	testUser = unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "testuser"})
	assert.True(t, testUser.MustChangePassword)
	testUserExclude = unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "testuserexclude"})
	assert.True(t, testUserExclude.MustChangePassword)

	// Reset password change flag by listing multiple users
	err = microcmdUserMustChangePassword().Run(t.Context(), []string{"change-test", "--unset", "testuser", "testuserexclude"})
	require.NoError(t, err)

	testUser = unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "testuser"})
	assert.False(t, testUser.MustChangePassword)
	testUserExclude = unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "testuserexclude"})
	assert.False(t, testUserExclude.MustChangePassword)

	// Exclude a user from all user
	err = microcmdUserMustChangePassword().Run(t.Context(), []string{"change-test", "--all", "--exclude", "testuserexclude"})
	require.NoError(t, err)

	testUser = unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "testuser"})
	assert.True(t, testUser.MustChangePassword)
	testUserExclude = unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "testuserexclude"})
	assert.False(t, testUserExclude.MustChangePassword)

	// Unset a flag for single user
	err = microcmdUserMustChangePassword().Run(t.Context(), []string{"change-test", "--unset", "testuser"})
	require.NoError(t, err)

	testUser = unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "testuser"})
	assert.False(t, testUser.MustChangePassword)
	testUserExclude = unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "testuserexclude"})
	assert.False(t, testUserExclude.MustChangePassword)
}
