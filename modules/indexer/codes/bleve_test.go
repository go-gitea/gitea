// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package codes

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIndexAndSearch(t *testing.T) {
	dir := "./bleve.index"
	indexer := NewBleveIndexer(dir)
	defer os.RemoveAll(dir)

	_, err := indexer.Init()
	assert.NoError(t, err)

	err = indexer.Index([]*IndexerData{
		{
			RepoID:  2,
			Content: "As title",
		},
		{
			RepoID:  2,
			Content: "Chinese Korean and Japanese should be supported but I would like it's not enabled by default",
		},
	})
	assert.NoError(t, err)

	var (
		keywords = []struct {
			Keyword string
			IDs     []int64
		}{
			{
				Keyword: "search",
				IDs:     []int64{1},
			},
			{
				Keyword: "test1",
				IDs:     []int64{1},
			},
			{
				Keyword: "test2",
				IDs:     []int64{1},
			},
			{
				Keyword: "support",
				IDs:     []int64{1, 2},
			},
			{
				Keyword: "chinese",
				IDs:     []int64{1, 2},
			},
			{
				Keyword: "help",
				IDs:     []int64{},
			},
		}
	)

	for _, kw := range keywords {
		res, err := indexer.Search(kw.IDs, kw.Keyword, 1, 10)
		assert.NoError(t, err)

		var ids = make([]int64, 0, len(res.Hits))
		for _, hit := range res.Hits {
			ids = append(ids, hit.RepoID)
		}
		assert.EqualValues(t, kw.IDs, ids)
	}
}
