// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"testing"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	secret_module "gitea.dev/modules/secret"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserSecretCRUD(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	require.ErrorIs(t, CreateUserSecret(t.Context(), user, "database_password", ""), ErrUserSecretValueInvalid)
	require.NoError(t, CreateUserSecret(t.Context(), user, "database_password", "initial-value"))
	require.ErrorIs(t, CreateUserSecret(t.Context(), user, "database_password", "another-value"), ErrUserSecretNameConflict)

	views, err := ListUserSecrets(t.Context(), user.ID)
	require.NoError(t, err)
	require.Len(t, views, 1)
	assert.Equal(t, "DATABASE_PASSWORD", views[0].Name)

	resolved, err := listUserSecretNames(t.Context(), user.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"DATABASE_PASSWORD"}, resolved)

	otherUserSecrets, err := listUserSecretNames(t.Context(), 4)
	require.NoError(t, err)
	assert.Empty(t, otherUserSecrets)

	require.NoError(t, UpdateUserSecretValue(t.Context(), user.ID, views[0].ID, "updated-value"))
	row := unittest.AssertExistsAndLoadBean(t, &codespace_model.UserSecret{ID: views[0].ID})
	value, err := secret_module.DecryptSecret(setting.SecretKey, row.DataEncrypted)
	require.NoError(t, err)
	assert.Equal(t, "updated-value", value)
	require.NoError(t, DeleteUserSecret(t.Context(), user.ID, views[0].ID))
	views, err = ListUserSecrets(t.Context(), user.ID)
	require.NoError(t, err)
	assert.Empty(t, views)
}

func TestUserSecretMutationsRequireOwner(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	otherUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	require.NoError(t, CreateUserSecret(t.Context(), owner, "owner_secret", "owner-value"))
	views, err := ListUserSecrets(t.Context(), owner.ID)
	require.NoError(t, err)
	require.Len(t, views, 1)
	secretID := views[0].ID

	require.ErrorIs(t, UpdateUserSecretValue(t.Context(), otherUser.ID, secretID, "changed"), ErrUserSecretNotFound)
	require.ErrorIs(t, DeleteUserSecret(t.Context(), otherUser.ID, secretID), ErrUserSecretNotFound)

	resolved, err := listUserSecretNames(t.Context(), owner.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"OWNER_SECRET"}, resolved)
}
