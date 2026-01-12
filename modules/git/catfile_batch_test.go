// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"io"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatFileBatch(t *testing.T) {
	defer test.MockVariableValue(&DefaultFeatures().SupportCatFileBatchCommand)()
	DefaultFeatures().SupportCatFileBatchCommand = false
	t.Run("LegacyCheck", testCatFileBatch)
	DefaultFeatures().SupportCatFileBatchCommand = true
	t.Run("BatchCommand", testCatFileBatch)
}

func testCatFileBatch(t *testing.T) {
	t.Run("CorruptedGitRepo", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := NewBatch(t.Context(), tmpDir)
		require.Error(t, err)
	})

	batch, err := NewBatch(t.Context(), filepath.Join(testReposDir, "repo1_bare"))
	require.NoError(t, err)
	defer batch.Close()

	t.Run("QueryInfo", func(t *testing.T) {
		info, err := batch.QueryInfo("e2129701f1a4d54dc44f03c93bca0a2aec7c5449")
		require.NoError(t, err)
		assert.Equal(t, "e2129701f1a4d54dc44f03c93bca0a2aec7c5449", info.ID)
		assert.Equal(t, "blob", info.Type)
		assert.EqualValues(t, 6, info.Size)
	})

	t.Run("QueryContent", func(t *testing.T) {
		info, rd, err := batch.QueryContent("e2129701f1a4d54dc44f03c93bca0a2aec7c5449")
		require.NoError(t, err)
		assert.Equal(t, "e2129701f1a4d54dc44f03c93bca0a2aec7c5449", info.ID)
		assert.Equal(t, "blob", info.Type)
		assert.EqualValues(t, 6, info.Size)

		content, err := io.ReadAll(io.LimitReader(rd, info.Size))
		require.NoError(t, err)
		require.Equal(t, "file1\n", string(content))
	})
}
