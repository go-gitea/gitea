// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

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
	} {
		exists, err := x.Dialect().IsTableExist(x.DB(), context.Background(), table)
		require.NoError(t, err)
		assert.True(t, exists, "table %s should exist", table)
	}

	codespaceIndexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "codespace")
	require.NoError(t, err)
	assert.True(t, hasIndex(codespaceIndexes, "user_id", "status"))
	assert.True(t, hasIndex(codespaceIndexes, "repo_id", "status"))
	assert.True(t, hasIndex(codespaceIndexes, "status", "operation_type", "operation_status", "manager_id", "repo_tag", "operation_created_unix", "uuid"))
	assert.True(t, hasIndex(codespaceIndexes, "manager_id", "operation_type", "operation_status", "status", "operation_created_unix", "uuid"))
	assert.True(t, hasIndex(codespaceIndexes, "operation_status", "operation_created_unix", "uuid"))
	assert.True(t, hasIndex(codespaceIndexes, "operation_status", "operation_deadline_unix", "uuid"))
	assert.True(t, hasIndex(codespaceIndexes, "status", "updated_unix", "uuid"))

	managerIndexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "codespace_manager")
	require.NoError(t, err)
	assert.True(t, hasIndex(managerIndexes, "owner_id", "runtime_state"))
	assert.True(t, hasIndex(managerIndexes, "runtime_state", "last_online_unix"))

	addressIndexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "codespace_manager_address")
	require.NoError(t, err)
	assert.True(t, hasUniqueIndex(addressIndexes, "manager_id", "kind"))
	assert.True(t, hasUniqueIndex(addressIndexes, "kind", "address"))

	managerTokenIndexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "codespace_manager_token")
	require.NoError(t, err)
	assert.True(t, hasUniqueIndex(managerTokenIndexes, "token"))
	assert.True(t, hasUniqueIndex(managerTokenIndexes, "owner_id"))

	giteaTokenIndexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "codespace_gitea_token")
	require.NoError(t, err)
	assert.True(t, hasUniqueIndex(giteaTokenIndexes, "token_hash"))

	sshKeyIndexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "codespace_ssh_key")
	require.NoError(t, err)
	assert.True(t, hasUniqueIndex(sshKeyIndexes, "key_id"))
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
