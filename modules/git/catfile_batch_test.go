// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

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

	simulateQueryTerminated := func(pipeCloseDelay, pipeReadDelay time.Duration) (errRead error) {
		catFileBatchDebugWaitClose.Store(int64(pipeCloseDelay))
		defer catFileBatchDebugWaitClose.Store(0)
		batch, err := NewBatch(t.Context(), filepath.Join(testReposDir, "repo1_bare"))
		require.NoError(t, err)
		defer batch.Close()
		_, _ = batch.QueryInfo("e2129701f1a4d54dc44f03c93bca0a2aec7c5449")
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
		}

		wg := sync.WaitGroup{}
		wg.Go(func() {
			time.Sleep(pipeReadDelay)
			var n int
			n, errRead = c.respReader.Read(make([]byte, 100))
			assert.Zero(t, n)
		})
		time.Sleep(10 * time.Millisecond)
		c.debugGitCmd.DebugKill()
		wg.Wait()
		return errRead
	}

	t.Run("QueryTerminated", func(t *testing.T) {
		err := simulateQueryTerminated(0, 20*time.Millisecond)
		assert.ErrorIs(t, err, os.ErrClosed) // pipes are closed faster
		err = simulateQueryTerminated(40*time.Millisecond, 20*time.Millisecond)
		assert.ErrorIs(t, err, io.EOF) // reader is faster
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
