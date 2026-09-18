// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"sync"
	"sync/atomic"
	"testing"

	"gitea.dev/models/unittest"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetOrCreateKeyPairGeneratesOnceConcurrently(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	var generated atomic.Int32
	generate := func() (string, string, error) {
		generated.Add(1)
		return util.GenerateKeyPair(2048)
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			<-start
			_, _, err := GetOrCreateKeyPair(t.Context(), 2, "test.key.private", "test.key.public", generate)
			assert.NoError(t, err)
		})
	}
	close(start)
	wg.Wait()
	assert.EqualValues(t, 1, generated.Load())
}
