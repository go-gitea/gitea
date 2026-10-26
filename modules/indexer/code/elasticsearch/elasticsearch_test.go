// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gitea.dev/models/db"
	"gitea.dev/modules/indexer/code/internal"
	es "gitea.dev/modules/indexer/internal/elasticsearch"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIndexPos(t *testing.T) {
	startIdx, endIdx := contentMatchIndexPos("test index start and end", "start", "end")
	assert.Equal(t, 11, startIdx)
	assert.Equal(t, 15, endIdx)
}

func TestContentAnalyzerIndexesUnicodeWords(t *testing.T) {
	esURL := test.ExternalServiceHTTP(t, "TEST_ELASTICSEARCH_URL", "http://elasticsearch:9200")
	indexName := fmt.Sprintf("gitea_test_%d", time.Now().UnixNano())
	indexer := NewIndexer(esURL, indexName)
	_, err := indexer.Init(t.Context())
	require.NoError(t, err)
	t.Cleanup(indexer.Close)
	t.Cleanup(func() { _ = indexer.Delete(context.Background(), 1) })

	const content = "Поставщик НомерДокумента SELECT Товары"
	require.NoError(t, indexer.Bulk(t.Context(), []es.BulkOp{
		es.IndexOp(internal.FilenameIndexerID(1, "unicode.txt"), map[string]any{
			"repo_id":    1,
			"filename":   "unicode.txt",
			"content":    content,
			"commit_id":  "test",
			"language":   "Text",
			"updated_at": time.Now().Unix(),
		}),
	}))
	require.NoError(t, indexer.Refresh(t.Context()))

	for _, keyword := range []string{"Поставщик", "НомерДокумента", "SELECT", "Товары"} {
		t.Run(keyword, func(t *testing.T) {
			total, results, _, err := indexer.Search(t.Context(), &internal.SearchOptions{
				RepoIDs:   []int64{1},
				Keyword:   keyword,
				Paginator: &db.ListOptions{Page: 1, PageSize: 10},
			})
			require.NoError(t, err)
			require.EqualValues(t, 1, total)
			require.Len(t, results, 1)
			assert.Equal(t, "unicode.txt", results[0].Filename)
		})
	}
}
