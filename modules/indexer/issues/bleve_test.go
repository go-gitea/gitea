// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issues

import (
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"github.com/stretchr/testify/assert"
)

func TestIndexAndSearch(t *testing.T) {
	indexer := NewBleveIndexer(setting.Indexer.IssuePath)
	_, err := indexer.Init()
	assert.NoError(t, err)

	err = indexer.Index([]*IndexerData{
		{
			ID:      1,
			RepoID:  2,
			Title:   "Issue search should support Chinese",
			Content: "As title",
		},
	})
	assert.NoError(t, err)

	res, err := indexer.Search("search", 2, 10, 0)
	assert.NoError(t, err)

	for _, hit := range res.Hits {
		assert.EqualValues(t, 1, hit.ID)
	}
}
