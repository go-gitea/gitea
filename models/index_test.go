// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResourceIndex(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			testInsertIssue(t, fmt.Sprintf("issue %d", i+1), "my issue", 0)
			wg.Done()
		}(i)
	}
	wg.Wait()
}
