// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package code

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestBleveIndexAndSearch(t *testing.T) {
	unittest.PrepareTestEnv(t)

	dir := t.TempDir()

	idx, _, err := NewBleveIndexer(dir)
	if err != nil {
		assert.Fail(t, "Unable to create bleve indexer Error: %v", err)
		if idx != nil {
			idx.Close()
		}
		return
	}
	defer idx.Close()

	testIndexer("beleve", t, idx)
}
