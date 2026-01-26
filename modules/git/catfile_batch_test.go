// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"io"
	"path/filepath"
	"sync"
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
		batch, err := NewBatch(t.Context(), tmpDir)
		// as long as the directory exists, no error, because we can't really know whether the git repo is valid until we run commands
		require.NoError(t, err)
		defer batch.Close()

		_, err = batch.QueryInfo("e2129701f1a4d54dc44f03c93bca0a2aec7c5449")
		require.Error(t, err)
		_, err = batch.QueryInfo("e2129701f1a4d54dc44f03c93bca0a2aec7c5449")
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

	t.Run("QueryTerminated", func(t *testing.T) {
		var c *catFileBatchCommunicator
		switch b := batch.(type) {
		case *catFileBatchLegacy:
			c = b.batchCheck
			_, _ = c.reqWriter.Write([]byte("in-complete-line-"))
		case *catFileBatchCommand:
			c = b.batch
			_, _ = c.reqWriter.Write([]byte("info"))
		default:
			t.FailNow()
			return
		}

		wg := sync.WaitGroup{}
		wg.Go(func() {
			buf := make([]byte, 100)
			_, _ = c.respReader.Read(buf)
			n, errRead := c.respReader.Read(buf)
			assert.Zero(t, n)
			assert.ErrorIs(t, errRead, io.EOF) // the pipe is closed due to command being killed
		})
		c.debugGitCmd.DebugKill()
		wg.Wait()
	})
}
