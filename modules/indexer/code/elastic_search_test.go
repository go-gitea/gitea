// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package code

import (
	"os"
	"testing"

	"code.gitea.io/gitea/models"

	"github.com/stretchr/testify/assert"
)

func TestESIndexAndSearch(t *testing.T) {
	models.PrepareTestEnv(t)

	u := os.Getenv("TEST_INDEXER_CODE_ES_URL")
	if u == "" {
		t.SkipNow()
		return
	}

	indexer, _, err := NewElasticSearchIndexer(u, "gitea_codes")
	if err != nil {
		assert.Fail(t, "Unable to create ES indexer Error: %v", err)
		if indexer != nil {
			indexer.Close()
		}
		return
	}
	defer indexer.Close()

	testIndexer("elastic_search", t, indexer)
}
