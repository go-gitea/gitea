// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bleve

import (
	"testing"
	"time"

	"gitea.dev/models/db"
	"gitea.dev/modules/indexer/code/internal"
	inner_bleve "gitea.dev/modules/indexer/internal/bleve"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBleveIndexerTokenFilter(t *testing.T) {
	dir := t.TempDir()
	indexer := NewIndexer(dir)
	defer indexer.Close()

	_, err := indexer.Init(t.Context())
	require.NoError(t, err)

	batch := inner_bleve.NewFlushingBatch(indexer.inner.Indexer, maxBatchSize)
	batch.Index("2", &RepoIndexerData{RepoID: 2, Content: "mDNS.port2=12345", UpdatedAt: time.Now()})
	batch.Flush()

	testCases := []struct {
		keyword     string
		expectedIDs []int64
	}{
		{keyword: "12345", expectedIDs: []int64{2}},
		{keyword: "DNS", expectedIDs: []int64{}},
		{keyword: "mdns", expectedIDs: []int64{2}},
		{keyword: "port", expectedIDs: []int64{2}},
		{keyword: "port2", expectedIDs: []int64{2}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.keyword, func(t *testing.T) {
			_, results, _, err := indexer.Search(t.Context(), &internal.SearchOptions{
				Paginator: &db.ListOptions{Page: 1, PageSize: 1},
				Keyword:   testCase.keyword,
			})
			require.NoError(t, err)
			assert.ElementsMatch(t, testCase.expectedIDs, searchResultIDs(results))
		})
	}
}

func searchResultIDs(result []*internal.SearchResult) []int64 {
	ids := make([]int64, 0, len(result))
	for _, hit := range result {
		ids = append(ids, hit.RepoID)
	}
	return ids
}
