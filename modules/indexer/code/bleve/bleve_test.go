// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bleve

import (
	"strconv"
	"testing"
	"time"

	"gitea.dev/models/db"
	indexer_module "gitea.dev/modules/indexer"
	"gitea.dev/modules/indexer/code/internal"
	inner_bleve "gitea.dev/modules/indexer/internal/bleve"
	"gitea.dev/modules/util"

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
	batch.Index("2", &RepoIndexerData{RepoID: 2, Filename: ".sub-dir/.sub-filename", Content: "mDNS.port2=12345", UpdatedAt: time.Now()})
	batch.Flush()

	testCases := []struct {
		keyword     string
		expectedIDs []int64
	}{
		{keyword: "12345", expectedIDs: []int64{2}},
		{keyword: "DNS", expectedIDs: []int64{2}},
		{keyword: "mdns", expectedIDs: []int64{2}},
		{keyword: "port", expectedIDs: []int64{2}},
		{keyword: "port2", expectedIDs: []int64{2}},
		{keyword: ".sub-dir", expectedIDs: []int64{2}},
		{keyword: "sub-file", expectedIDs: []int64{2}}, // prefix matching? not sure whether this behavior is wanted
		{keyword: "filename", expectedIDs: []int64{2}},
		{keyword: "name", expectedIDs: []int64{}},
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

func TestBleveIndexerPagesVerifiedCandidates(t *testing.T) {
	indexer := NewIndexer(t.TempDir())
	defer indexer.Close()
	_, err := indexer.Init(t.Context())
	require.NoError(t, err)

	batch := inner_bleve.NewFlushingBatch(indexer.inner.Indexer, maxBatchSize)
	for i, content := range []string{"abc bcd cde def", "abcdef", "abc bcd cde def", "abcdef", "abc bcd cde def"} {
		filename := strconv.Itoa(i)
		require.NoError(t, batch.Index(internal.FilenameIndexerID(1, filename), &RepoIndexerData{RepoID: 1, Filename: filename, Content: content, UpdatedAt: time.Now()}))
	}
	require.NoError(t, batch.Flush())

	var filenames []string
	for page := 1; page <= 3; page++ {
		total, results, _, err := indexer.Search(t.Context(), &internal.SearchOptions{
			Paginator:  &db.ListOptions{Page: page, PageSize: 1},
			Keyword:    "abcdef",
			SearchMode: indexer_module.SearchModeExact,
		})
		require.NoError(t, err)
		require.Len(t, results, util.Iif(page < 3, 1, 0))
		if page == 3 {
			assert.EqualValues(t, 2, total)
		}
		for _, result := range results {
			filenames = append(filenames, result.Filename)
		}
	}
	assert.ElementsMatch(t, []string{"1", "3"}, filenames)
}

func searchResultIDs(result []*internal.SearchResult) []int64 {
	ids := make([]int64, 0, len(result))
	for _, hit := range result {
		ids = append(ids, hit.RepoID)
	}
	return ids
}
