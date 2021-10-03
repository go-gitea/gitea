// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package code

import (
	"os"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestBleveIndexAndSearch(t *testing.T) {
	db.PrepareTestEnv(t)

	dir, err := os.MkdirTemp("", "bleve.index")
	assert.NoError(t, err)
	if err != nil {
		assert.Fail(t, "Unable to create temporary directory")
		return
	}
	defer util.RemoveAll(dir)

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
