// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getBatchCommunicators(t *testing.T, batch CatFileBatchCloser) []*catFileBatchCommunicator {
	t.Helper()
	switch b := batch.(type) {
	case *catFileBatchCommand:
		if b.batch == nil {
			return nil
		}
		return []*catFileBatchCommunicator{b.batch}
	case *catFileBatchLegacy:
		ret := make([]*catFileBatchCommunicator, 0, 2)
		if b.batchCheck != nil {
			ret = append(ret, b.batchCheck)
		}
		if b.batchContent != nil {
			ret = append(ret, b.batchContent)
		}
		return ret
	default:
		t.Fatalf("unexpected batch type %T", batch)
		return nil
	}
}

func TestRepoCatFileBatch(t *testing.T) {
	t.Run("MissingRepoAndClose", func(t *testing.T) {
		repo, err := OpenRepository(t.Context(), filepath.Join(testReposDir, "repo1_bare"))
		require.NoError(t, err)
		repo.Path = "/no-such" // when the repo is missing (it usually occurs during testing because the fixtures are synced frequently)
		_, _, err = repo.CatFileBatch(t.Context())
		require.Error(t, err)
		require.NoError(t, repo.Close()) // shouldn't panic
	})

	t.Run("CloseCleansUpTemporaryBatch", func(t *testing.T) {
		repo, err := OpenRepository(t.Context(), filepath.Join(testReposDir, "repo1_bare"))
		require.NoError(t, err)

		sharedBatch, sharedCancel, err := repo.CatFileBatch(t.Context())
		require.NoError(t, err)
		defer sharedCancel()

		tempBatch, tempCancel, err := repo.CatFileBatch(t.Context())
		require.NoError(t, err)

		_, err = sharedBatch.QueryInfo("e2129701f1a4d54dc44f03c93bca0a2aec7c5449")
		require.NoError(t, err)
		_, err = tempBatch.QueryInfo("e2129701f1a4d54dc44f03c93bca0a2aec7c5449")
		require.NoError(t, err)

		require.Len(t, repo.tempCatFileBatchStore, 1)
		communicators := getBatchCommunicators(t, tempBatch.(CatFileBatchCloser))
		require.NotEmpty(t, communicators)
		for _, c := range communicators {
			require.NotNil(t, c.closeFunc.Load())
		}

		require.NoError(t, repo.Close())
		assert.Nil(t, repo.tempCatFileBatchStore)
		for _, c := range communicators {
			assert.Nil(t, c.closeFunc.Load())
		}

		// The repo cleanup should already have released the temporary batch.
		tempCancel()
	})

	t.Run("TemporaryBatchCloseRemovesTracking", func(t *testing.T) {
		repo, err := OpenRepository(t.Context(), filepath.Join(testReposDir, "repo1_bare"))
		require.NoError(t, err)
		defer func() { require.NoError(t, repo.Close()) }()

		sharedBatch, sharedCancel, err := repo.CatFileBatch(t.Context())
		require.NoError(t, err)
		defer sharedCancel()

		tempBatch, tempCancel, err := repo.CatFileBatch(t.Context())
		require.NoError(t, err)

		_, err = sharedBatch.QueryInfo("e2129701f1a4d54dc44f03c93bca0a2aec7c5449")
		require.NoError(t, err)
		_, err = tempBatch.QueryInfo("e2129701f1a4d54dc44f03c93bca0a2aec7c5449")
		require.NoError(t, err)

		require.Len(t, repo.tempCatFileBatchStore, 1)
		communicators := getBatchCommunicators(t, tempBatch.(CatFileBatchCloser))
		require.NotEmpty(t, communicators)

		tempCancel()
		assert.Empty(t, repo.tempCatFileBatchStore)
		for _, c := range communicators {
			assert.Nil(t, c.closeFunc.Load())
		}

		// Closing again should stay a no-op.
		tempCancel()
		assert.Empty(t, repo.tempCatFileBatchStore)
	})

	// TODO: test more methods and concurrency queries
}
