// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_28

import (
	"context"
	"slices"
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm/schemas"
)

func Test_AddCodespaceTables(t *testing.T) {
	x, deferable := migrationtest.PrepareTestEnv(t, 0)
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	require.NoError(t, AddCodespaceTables(x))

	for _, table := range []string{
		"codespace",
		"codespace_manager",
		"codespace_manager_address",
		"codespace_manager_token",
		"codespace_gitea_token",
		"codespace_ssh_key",
		"codespace_permission_authorization",
		"codespace_permission_repository",
		"codespace_user_secret",
		"codespace_user_secret_repository",
	} {
		exists, err := x.Dialect().IsTableExist(x.DB(), context.Background(), table)
		require.NoError(t, err)
		assert.True(t, exists, "table %s should exist", table)
	}

	codespaceIndexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "codespace")
	require.NoError(t, err)
	assert.True(t, hasIndex(codespaceIndexes, "user_id", "updated_unix", "created_unix"))
	assert.True(t, hasIndex(codespaceIndexes, "repo_id"))
	assert.True(t, hasIndex(codespaceIndexes, "status", "operation_type", "operation_status", "manager_id", "environment_tag", "operation_created_unix", "uuid"))
	assert.True(t, hasIndex(codespaceIndexes, "manager_id", "operation_status", "operation_type", "status", "operation_created_unix", "uuid"))
	assert.True(t, hasIndex(codespaceIndexes, "operation_status", "operation_created_unix", "uuid"))
	assert.True(t, hasIndex(codespaceIndexes, "operation_status", "operation_deadline_unix", "uuid"))
	assert.True(t, hasIndex(codespaceIndexes, "status", "updated_unix", "uuid"))

	managerIndexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "codespace_manager")
	require.NoError(t, err)
	assert.True(t, hasIndex(managerIndexes, "user_id"))

	addressIndexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "codespace_manager_address")
	require.NoError(t, err)
	assert.True(t, hasUniqueIndex(addressIndexes, "manager_id", "kind"))
	assert.True(t, hasUniqueIndex(addressIndexes, "kind", "address"))

	managerTokenIndexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "codespace_manager_token")
	require.NoError(t, err)
	assert.True(t, hasUniqueIndex(managerTokenIndexes, "token"))
	assert.True(t, hasUniqueIndex(managerTokenIndexes, "user_id"))

	giteaTokenIndexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "codespace_gitea_token")
	require.NoError(t, err)
	assert.True(t, hasUniqueIndex(giteaTokenIndexes, "token_hash"))

	sshKeyIndexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "codespace_ssh_key")
	require.NoError(t, err)
	assert.True(t, hasUniqueIndex(sshKeyIndexes, "key_id"))

	authorizationIndexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "codespace_permission_authorization")
	require.NoError(t, err)
	assert.True(t, hasIndex(authorizationIndexes, "user_id", "source_repo_id", "request_hash"))

	repositoryIndexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "codespace_permission_repository")
	require.NoError(t, err)
	assert.True(t, hasUniqueIndex(repositoryIndexes, "authorization_id", "target_repo_id", "unit_type"))

	secretIndexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "codespace_user_secret")
	require.NoError(t, err)
	assert.True(t, hasUniqueIndex(secretIndexes, "user_id", "name"))
	_, secretColumns, err := x.Dialect().GetColumns(x.DB(), context.Background(), "codespace_user_secret")
	require.NoError(t, err)
	assert.Contains(t, secretColumns, "all_repositories")

	secretRepositoryIndexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "codespace_user_secret_repository")
	require.NoError(t, err)
	assert.True(t, hasIndex(secretRepositoryIndexes, "secret_id"))
	assert.True(t, hasIndex(secretRepositoryIndexes, "repo_id"))
	assert.True(t, hasUniqueIndex(secretRepositoryIndexes, "secret_id", "repo_id"))
}

func hasUniqueIndex(indexes map[string]*schemas.Index, columns ...string) bool {
	for _, index := range indexes {
		if index.Type == schemas.UniqueType && slices.Equal(index.Cols, columns) {
			return true
		}
	}
	return false
}

func hasIndex(indexes map[string]*schemas.Index, columns ...string) bool {
	for _, index := range indexes {
		if index.Type == schemas.IndexType && slices.Equal(index.Cols, columns) {
			return true
		}
	}
	return false
}
