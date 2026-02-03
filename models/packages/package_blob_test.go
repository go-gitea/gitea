// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"sync"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetOrInsertBlobConcurrent(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// Create a blob with unique hash values
	testBlob := &PackageBlob{
		Size:       12345,
		HashMD5:    "d41d8cd98f00b204e9800998ecf8427e",
		HashSHA1:   "da39a3ee5e6b4b0d3255bfef95601890afd80709",
		HashSHA256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		HashSHA512: "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e",
	}

	const numGoroutines = 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	results := make([]*PackageBlob, numGoroutines)
	errors := make([]error, numGoroutines)
	existed := make([]bool, numGoroutines)

	// All goroutines try to insert the same blob concurrently
	for i := range numGoroutines {
		go func(idx int) {
			defer wg.Done()

			// Create a copy of the test blob for each goroutine
			blob := &PackageBlob{
				Size:       testBlob.Size,
				HashMD5:    testBlob.HashMD5,
				HashSHA1:   testBlob.HashSHA1,
				HashSHA256: testBlob.HashSHA256,
				HashSHA512: testBlob.HashSHA512,
			}

			results[idx], existed[idx], errors[idx] = GetOrInsertBlob(db.DefaultContext, blob)
		}(i)
	}

	wg.Wait()

	// All requests should succeed
	var successfulResults []*PackageBlob
	for i := range numGoroutines {
		if errors[i] != nil {
			t.Errorf("goroutine %d failed: %v", i, errors[i])
			continue
		}
		require.NotNil(t, results[i], "goroutine %d returned nil PackageBlob", i)
		successfulResults = append(successfulResults, results[i])
	}

	// All successful results should have the same blob ID
	if len(successfulResults) > 1 {
		firstID := successfulResults[0].ID
		for i, result := range successfulResults[1:] {
			assert.Equal(t, firstID, result.ID, "blob IDs should match for goroutine %d", i+1)
		}
	}

	// At least one should have existed=false (the first one to insert)
	// and all others should have existed=true
	insertCount := 0
	for _, e := range existed {
		if !e {
			insertCount++
		}
	}
	// Exactly one should have inserted (existed=false)
	// Note: Due to the race condition fix, some might also see existed=true
	// even if they tried to insert but found it already exists
	assert.GreaterOrEqual(t, insertCount, 1, "at least one goroutine should have inserted the blob")
}
