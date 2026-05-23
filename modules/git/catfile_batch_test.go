// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"io"
	"os"
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
		batch, err := NewBatch(t.Context(), tmpDir)
		// as long as the directory exists, no error, because we can't really know whether the git repo is valid until we run commands
		require.NoError(t, err)
		defer batch.Close()

		_, err = batch.QueryInfo("e2129701f1a4d54dc44f03c93bca0a2aec7c5449")
		require.Error(t, err)
		_, err = batch.QueryInfo("e2129701f1a4d54dc44f03c93bca0a2aec7c5449")
		require.Error(t, err)
	})

	simulateQueryTerminated := func(t *testing.T, errBeforePipeClose, errAfterPipeClose error) {
		readError := func(t *testing.T, r io.Reader, expectedErr error) {
			if expectedErr == nil {
				return // expectedErr == nil means this read should be skipped
			}
			n, err := r.Read(make([]byte, 100))
			assert.Zero(t, n)
			assert.ErrorIs(t, err, expectedErr)
		}

		batch, err := NewBatch(t.Context(), filepath.Join(testReposDir, "repo1_bare"))
		require.NoError(t, err)
		defer batch.Close()
		_, err = batch.QueryInfo("e2129701f1a4d54dc44f03c93bca0a2aec7c5449")
		require.NoError(t, err)

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

		require.NotEqual(t, errBeforePipeClose == nil, errAfterPipeClose == nil, "must set exactly one of the expected errors")

		inceptor := c.debugKill()
		<-inceptor.beforeClose                         // wait for the command's Close to be called, the pipe is not closed yet
		readError(t, c.respReader, errBeforePipeClose) // then caller will read on an open pipe which will be closed soon
		close(inceptor.blockClose)                     // continue to close the pipe
		<-inceptor.afterClose                          // wait for the pipe to be closed
		readError(t, c.respReader, errAfterPipeClose)  // then caller will read on a closed pipe
	}
	t.Run("QueryTerminated", func(t *testing.T) {
		simulateQueryTerminated(t, io.EOF, nil)       // reader is faster
		simulateQueryTerminated(t, nil, os.ErrClosed) // pipes are closed faster
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
