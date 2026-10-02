// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"strconv"
	"strings"
	"testing"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserSecretRepositoryScopeAndValueUpdate(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	require.NoError(t, CreateUserSecret(t.Context(), user, "database_password", "initial-value", false, []int64{repo.ID}))

	views, err := ListUserSecrets(t.Context(), user.ID)
	require.NoError(t, err)
	require.Len(t, views, 1)
	assert.Equal(t, "DATABASE_PASSWORD", views[0].Name)
	require.Len(t, views[0].Repositories, 1)
	assert.Equal(t, repo.ID, views[0].Repositories[0].ID)

	resolved, err := resolveUserSecretsForRepository(t.Context(), user.ID, repo.ID)
	require.NoError(t, err)
	assert.Equal(t, []RuntimeSecret{{Name: "DATABASE_PASSWORD", Value: "initial-value"}}, resolved)

	otherRepoSecrets, err := resolveUserSecretsForRepository(t.Context(), user.ID, 2)
	require.NoError(t, err)
	assert.Empty(t, otherRepoSecrets)

	require.NoError(t, UpdateUserSecretValue(t.Context(), user.ID, views[0].ID, "updated-value"))
	resolved, err = resolveUserSecretsForRepository(t.Context(), user.ID, repo.ID)
	require.NoError(t, err)
	assert.Equal(t, "updated-value", resolved[0].Value)
}

func TestUserSecretRepositoryAccessModes(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	require.NoError(t, CreateUserSecret(t.Context(), user, "personal_token", "secret-value", false, nil))
	views, err := ListUserSecrets(t.Context(), user.ID)
	require.NoError(t, err)
	require.Len(t, views, 1)
	assert.False(t, views[0].AllRepositories)
	assert.Empty(t, views[0].Repositories)

	require.NoError(t, UpdateUserSecretRepositoryAccess(t.Context(), user, views[0].ID, true, nil))
	resolved, err := resolveUserSecretsForRepository(t.Context(), user.ID, 2)
	require.NoError(t, err)
	assert.Equal(t, []RuntimeSecret{{Name: "PERSONAL_TOKEN", Value: "secret-value"}}, resolved)
	recommendations, available, err := resolveCreateSecrets(t.Context(), user.ID, 2, []CreateRecommendedSecret{{Name: "PERSONAL_TOKEN"}})
	require.NoError(t, err)
	assert.True(t, recommendations[0].Configured)
	assert.True(t, recommendations[0].Available)
	assert.Equal(t, []CreateSecretSummary{{Name: "PERSONAL_TOKEN"}}, available)

	require.NoError(t, UpdateUserSecretRepositoryAccess(t.Context(), user, views[0].ID, false, []int64{1}))
	resolved, err = resolveUserSecretsForRepository(t.Context(), user.ID, 2)
	require.NoError(t, err)
	assert.Empty(t, resolved)
	resolved, err = resolveUserSecretsForRepository(t.Context(), user.ID, 1)
	require.NoError(t, err)
	assert.Len(t, resolved, 1)
}

func TestUserSecretMutationsRequireOwner(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	otherUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	require.NoError(t, CreateUserSecret(t.Context(), owner, "owner_secret", "owner-value", false, []int64{1}))
	views, err := ListUserSecrets(t.Context(), owner.ID)
	require.NoError(t, err)
	require.Len(t, views, 1)
	secretID := views[0].ID

	require.ErrorIs(t, UpdateUserSecretValue(t.Context(), otherUser.ID, secretID, "changed"), ErrUserSecretNotFound)
	require.ErrorIs(t, UpdateUserSecretRepositoryAccess(t.Context(), otherUser, secretID, true, nil), ErrUserSecretNotFound)
	require.ErrorIs(t, DeleteUserSecret(t.Context(), otherUser.ID, secretID), ErrUserSecretNotFound)

	resolved, err := resolveUserSecretsForRepository(t.Context(), owner.ID, 1)
	require.NoError(t, err)
	assert.Equal(t, []RuntimeSecret{{Name: "OWNER_SECRET", Value: "owner-value"}}, resolved)
}

func TestUserSecretSizeLimitIsPerUser(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	otherSecret := &codespace_model.UserSecret{UserID: 1, Name: "OTHER_USER_SECRET", DataEncrypted: "unused", DataSize: repositorySecretTotalSizeLimit}
	require.NoError(t, db.Insert(t.Context(), otherSecret))
	require.NoError(t, db.Insert(t.Context(), &codespace_model.UserSecretRepository{SecretID: otherSecret.ID, RepoID: 1}))

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	require.NoError(t, CreateUserSecret(t.Context(), user, "own_secret", strings.Repeat("x", 64), false, []int64{1}))
}

func TestUserSecretValidationErrors(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	require.ErrorIs(t, CreateUserSecret(t.Context(), user, "invalid_value", "", false, nil), ErrUserSecretValueInvalid)
	require.NoError(t, CreateUserSecret(t.Context(), user, "duplicate_name", "first", false, nil))
	require.ErrorIs(t, CreateUserSecret(t.Context(), user, "duplicate_name", "second", false, nil), ErrUserSecretNameConflict)
}

func TestUserSecretAllRepositoriesSizeLimit(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	value := strings.Repeat("x", userSecretValueSizeLimit)
	for i := range repositorySecretTotalSizeLimit / userSecretValueSizeLimit {
		require.NoError(t, CreateUserSecret(t.Context(), user, "secret_"+strconv.Itoa(i), value, true, nil))
	}
	require.ErrorIs(t, CreateUserSecret(t.Context(), user, "one_too_many", value, true, nil), ErrUserSecretSizeLimit)
}
