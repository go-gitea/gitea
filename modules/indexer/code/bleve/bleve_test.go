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

func TestBleveSearch(t *testing.T) {
	indexer := NewIndexer(t.TempDir())
	t.Cleanup(indexer.Close)
	_, err := indexer.Init(t.Context())
	require.NoError(t, err)

	batch := inner_bleve.NewFlushingBatch(indexer.inner.Indexer, maxBatchSize)
	for _, document := range []RepoIndexerData{
		{Filename: "hit.go", Content: "abcdef", Language: "Go"},
		{Filename: "false.go", Content: "abc bcd cde def", Language: "Go"},
		{Filename: "dir/a\nb.txt", Content: "needle"},
	} {
		document.RepoID = 9007199254740993
		document.UpdatedAt = time.Now().UTC()
		require.NoError(t, batch.Index(internal.FilenameIndexerID(document.RepoID, document.Filename), &document))
	}
	require.NoError(t, batch.Flush())

	for _, testCase := range []struct {
		name          string
		term          string
		path          string
		page          int
		total         int64
		filenames     []string
		languageCount int
	}{
		{name: "short term page past end", term: "abc", page: 3, total: 2, languageCount: 2},
		{name: "verified language count", term: "abcdef", page: 1, total: 1, filenames: []string{"hit.go"}, languageCount: 1},
		{name: "global directory with newline filename", term: "needle", path: "dir", page: 1, total: 1, filenames: []string{"dir/a\nb.txt"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			total, results, languages, err := indexer.Search(t.Context(), &internal.SearchOptions{
				Terms:     []string{testCase.term},
				Path:      testCase.path,
				Paginator: &db.ListOptions{Page: testCase.page, PageSize: 10},
			})
			require.NoError(t, err)
			assert.Equal(t, testCase.total, total)
			var filenames []string
			for _, result := range results {
				assert.EqualValues(t, 9007199254740993, result.RepoID)
				filenames = append(filenames, result.Filename)
			}
			assert.Equal(t, testCase.filenames, filenames)
			if testCase.languageCount > 0 {
				require.Len(t, languages, 1)
				assert.Equal(t, "Go", languages[0].Language)
				assert.Equal(t, testCase.languageCount, languages[0].Count)
			} else {
				assert.Empty(t, languages)
			}
		})
	}
}
