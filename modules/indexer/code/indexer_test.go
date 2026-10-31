// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package code

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/indexer/code/bleve"
	"gitea.dev/modules/indexer/code/internal"

	_ "gitea.dev/models"
	_ "gitea.dev/models/actions"
	_ "gitea.dev/models/activities"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) { unittest.MainTest(m) }

func TestIndexRepository(t *testing.T) {
	unittest.PrepareTestEnv(t)
	idx := bleve.NewIndexer(t.TempDir())
	_, err := idx.Init(t.Context())
	require.NoError(t, err)
	defer idx.Close()
	require.NoError(t, index(t.Context(), idx, 1))
	var indexer internal.Indexer = idx
	previousIndexer := globalIndexer.Load()
	defer globalIndexer.Store(previousIndexer)
	globalIndexer.Store(&indexer)
	total, results, _, err := PerformSearch(t.Context(), &SearchOptions{Terms: []string{"Description"}, Paginator: &db.ListOptions{Page: 1, PageSize: 1}})
	require.NoError(t, err)
	assert.Positive(t, total)
	require.Len(t, results, 1)
	assert.Equal(t, []*ResultFragment{{StartLine: 2, Content: "\nDescription for repo1", Matches: []MatchRange{{Start: 1, End: 12}}}}, BuildResultFragments(results[0].Content, results[0].ContentMatches))
	require.NoError(t, idx.Delete(t.Context(), 1))
	total, _, _, err = idx.Search(t.Context(), &internal.SearchOptions{Terms: []string{"Description"}, Paginator: &db.ListOptions{Page: 1, PageSize: 1}})
	require.NoError(t, err)
	assert.Zero(t, total)
}
