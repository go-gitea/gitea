// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bleve

import (
	"testing"

	"gitea.dev/modules/indexer/issues/internal"
	"gitea.dev/modules/indexer/issues/internal/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBleveIndexer(t *testing.T) {
	dir := t.TempDir()
	indexer := NewIndexer(dir)
	defer indexer.Close()

	tests.TestIndexer(t, indexer)
}

func TestBleveIndexerSearch(t *testing.T) {
	type searchCase struct {
		name        string
		opts        *internal.SearchOptions
		expectedIDs []int64
	}
	for _, testCase := range []struct {
		name       string
		data       []*internal.IndexerData
		searches   []searchCase
		checkTotal bool
	}{
		{
			name: "no assignee",
			data: []*internal.IndexerData{
				{ID: 1, Title: "assigned through assignee_ids", AssigneeIDs: []int64{2}},
				{ID: 2, Title: "unassigned", NoAssignee: true},
				{ID: 3, Title: "assigned through multiple assignee_ids", AssigneeIDs: []int64{3, 4}},
			},
			searches: []searchCase{
				{"none", &internal.SearchOptions{AssigneeID: "(none)"}, []int64{2}},
				{"any", &internal.SearchOptions{AssigneeID: "(any)"}, []int64{1, 3}},
				{"specific", &internal.SearchOptions{AssigneeID: "2"}, []int64{1}},
				{"specific first multi-assignee", &internal.SearchOptions{AssigneeID: "3"}, []int64{3}},
				{"specific second multi-assignee", &internal.SearchOptions{AssigneeID: "4"}, []int64{3}},
			},
			checkTotal: true,
		},
		{
			name: "token filter",
			data: []*internal.IndexerData{
				{ID: 1, Title: "fix(packages): SomeThing needs a rewrite (#12345)"},
				{ID: 2, Title: "add support for mDNS discovery abc1234"},
			},
			searches: []searchCase{
				{"exact original case", &internal.SearchOptions{Keyword: "SomeThing"}, []int64{1}},
				{"case matching original transitions", &internal.SearchOptions{Keyword: "someThing"}, []int64{1}},
				{"all lower case", &internal.SearchOptions{Keyword: "something"}, []int64{1}},
				{"all upper case", &internal.SearchOptions{Keyword: "SOMETHING"}, []int64{1}},
				{"number match", &internal.SearchOptions{Keyword: "12345"}, []int64{1}},
				{"number as part", &internal.SearchOptions{Keyword: "1234"}, []int64{2}},
				{"sub-word search still works", &internal.SearchOptions{Keyword: "DNS"}, []int64{2}},
				{"sub-word search, lower case", &internal.SearchOptions{Keyword: "mdns"}, []int64{2}},
				{"keyword is camel case", &internal.SearchOptions{Keyword: "addSupport"}, []int64{2}},
				{"keyword not match", &internal.SearchOptions{Keyword: "addsupport"}, []int64{}},
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			indexer := NewIndexer(t.TempDir())
			defer indexer.Close()

			_, err := indexer.Init(t.Context())
			require.NoError(t, err)
			require.NoError(t, indexer.Index(t.Context(), testCase.data...))

			for _, search := range testCase.searches {
				t.Run(search.name, func(t *testing.T) {
					result, err := indexer.Search(t.Context(), search.opts)
					require.NoError(t, err)
					if testCase.checkTotal {
						assert.Equal(t, int64(len(search.expectedIDs)), result.Total)
					}
					assert.ElementsMatch(t, search.expectedIDs, searchResultIDs(result))
				})
			}
		})
	}
}

func searchResultIDs(result *internal.SearchResult) []int64 {
	ids := make([]int64, 0, len(result.Hits))
	for _, hit := range result.Hits {
		ids = append(ids, hit.ID)
	}
	return ids
}
