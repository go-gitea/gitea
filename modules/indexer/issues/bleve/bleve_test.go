// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bleve

import (
	"context"
	"testing"

	"code.gitea.io/gitea/modules/indexer/issues/internal"

	"github.com/stretchr/testify/assert"
)

func TestBleveIndexAndSearch(t *testing.T) {
	dir := t.TempDir()
	indexer := NewIndexer(dir)
	defer indexer.Close()

	if _, err := indexer.Init(context.Background()); err != nil {
		assert.Fail(t, "Unable to initialize bleve indexer: %v", err)
		return
	}

	err := indexer.Index(context.Background(), []*internal.IndexerData{
		{
			ID:      1,
			RepoID:  2,
			Title:   "Issue search should support Chinese",
			Content: "As title",
			Comments: []string{
				"test1",
				"test2",
			},
		},
		{
			ID:      2,
			RepoID:  2,
			Title:   "CJK support could be optional",
			Content: "Chinese Korean and Japanese should be supported but I would like it's not enabled by default",
			Comments: []string{
				"LGTM",
				"Good idea",
			},
		},
	})
	assert.NoError(t, err)

	keywords := []struct {
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

	for _, kw := range keywords {
		res, err := indexer.Search(context.TODO(), kw.Keyword, []int64{2}, 10, 0)
		assert.NoError(t, err)

		ids := make([]int64, 0, len(res.Hits))
		for _, hit := range res.Hits {
			ids = append(ids, hit.ID)
		}
		assert.ElementsMatch(t, kw.IDs, ids)
	}
}
