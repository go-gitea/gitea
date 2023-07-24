// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bleve

import (
	"testing"

	"code.gitea.io/gitea/modules/indexer/issues/internal/tests"
)

func TestBleveIndexer(t *testing.T) {
	dir := t.TempDir()
	indexer := NewIndexer(dir)
	defer indexer.Close()

	tests.TestIndexer(t, indexer)
}
