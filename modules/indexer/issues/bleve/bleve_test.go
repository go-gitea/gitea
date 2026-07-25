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

func TestBleveIndexerNoAssignee(t *testing.T) {
	dir := t.TempDir()
	indexer := NewIndexer(dir)
	defer indexer.Close()

	_, err := indexer.Init(t.Context())
	require.NoError(t, err)

	require.NoError(t, indexer.Index(t.Context(),
		&internal.IndexerData{ID: 1, Title: "assigned through assignee_ids", AssigneeIDs: []int64{2}},
		&internal.IndexerData{ID: 2, Title: "unassigned", NoAssignee: true},
		&internal.IndexerData{ID: 3, Title: "assigned through multiple assignee_ids", AssigneeIDs: []int64{3, 4}},
	))

	testCases := []struct {
		name        string
		opts        *internal.SearchOptions
		expectedIDs []int64
	}{
		{
			name:        "none",
			opts:        &internal.SearchOptions{AssigneeID: "(none)"},
			expectedIDs: []int64{2},
		},
		{
			name:        "any",
			opts:        &internal.SearchOptions{AssigneeID: "(any)"},
			expectedIDs: []int64{1, 3},
		},
		{
			name:        "specific",
			opts:        &internal.SearchOptions{AssigneeID: "2"},
			expectedIDs: []int64{1},
		},
		{
			name:        "specific first multi-assignee",
			opts:        &internal.SearchOptions{AssigneeID: "3"},
			expectedIDs: []int64{3},
		},
		{
			name:        "specific second multi-assignee",
			opts:        &internal.SearchOptions{AssigneeID: "4"},
			expectedIDs: []int64{3},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := indexer.Search(t.Context(), testCase.opts)
			require.NoError(t, err)
			assert.Equal(t, int64(len(testCase.expectedIDs)), result.Total)
			assert.ElementsMatch(t, testCase.expectedIDs, searchResultIDs(result))
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
