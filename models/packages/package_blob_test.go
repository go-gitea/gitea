// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestGetOrInsertBlobConcurrent(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	testBlob := PackageBlob{
		Size:       123,
		HashMD5:    "md5",
		HashSHA1:   "sha1",
		HashSHA256: "sha256",
		HashSHA512: "sha512",
	}

	const numGoroutines = 3
	var wg errgroup.Group
	results := make([]*PackageBlob, numGoroutines)
	existed := make([]bool, numGoroutines)
	for idx := range numGoroutines {
		wg.Go(func() error {
			blob := testBlob // Create a copy of the test blob for each goroutine
			var err error
			results[idx], existed[idx], err = GetOrInsertBlob(t.Context(), &blob)
			return err
		})
	}
	require.NoError(t, wg.Wait())

	// then: all GetOrInsertBlob succeeds with the same blob ID, and only one indicates it did not exist before
	existedCount := 0
	assert.NotNil(t, results[0])
	for i := range numGoroutines {
		assert.Equal(t, results[0].ID, results[i].ID)
		if existed[i] {
			existedCount++
		}
	}
	assert.Equal(t, numGoroutines-1, existedCount)
}
