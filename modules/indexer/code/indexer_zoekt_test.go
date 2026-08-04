// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build zoekt && unix

package code

import (
	"testing"
	"time"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	indexer_module "gitea.dev/modules/indexer"
	"gitea.dev/modules/indexer/code/internal"
	"gitea.dev/modules/indexer/code/zoekt"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZoektIndexAndSearch(t *testing.T) {
	unittest.PrepareTestEnv(t)

	idx := zoekt.NewIndexer(t.TempDir())
	defer idx.Close()

	_, err := idx.Init(t.Context())
	require.NoError(t, err)

	require.NoError(t, setupRepositoryIndexes(t.Context(), idx))
	defer func() {
		assert.NoError(t, tearDownRepositoryIndexes(t.Context(), idx))
	}()

	search := func(t *testing.T, mode indexer_module.SearchModeType, keyword string, page, pageSize int) (int64, []string) {
		t.Helper()
		total, res, _, err := idx.Search(t.Context(), &internal.SearchOptions{
			Keyword:    keyword,
			SearchMode: mode,
			Paginator:  &db.ListOptions{Page: page, PageSize: pageSize},
		})
		require.NoError(t, err)
		filenames := make([]string, 0, len(res))
		for _, r := range res {
			filenames = append(filenames, r.Filename)
		}
		return total, filenames
	}

	// the searcher watches the index directory, give it a moment to pick up the shards
	require.Eventually(t, func() bool {
		total, _ := search(t, indexer_module.SearchModeExact, "cheese", 1, 10)
		return total == 2
	}, 10*time.Second, 100*time.Millisecond, "index did not become searchable")

	t.Run("only matching lines with context are returned", func(t *testing.T) {
		// README.md of repo 1 has three lines, the keyword matches only the third
		// one, so the result must contain the matching line plus one context line
		// instead of the whole file
		_, res, _, err := idx.Search(t.Context(), &internal.SearchOptions{
			Keyword:    "Description",
			SearchMode: indexer_module.SearchModeExact,
			Paginator:  &db.ListOptions{Page: 1, PageSize: 10},
		})
		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, "README.md", res[0].Filename)
		assert.Equal(t, "\nDescription for repo1", res[0].Content)
		assert.Equal(t, 2, res[0].ContentStartLineNum)
		assert.Equal(t, "Description", res[0].Content[res[0].StartIndex:res[0].EndIndex])
	})

	t.Run("exact keyword with quotes and parens", func(t *testing.T) {
		// the keyword is matched literally, none of its characters may be taken
		// as zoekt query syntax
		total, files := search(t, indexer_module.SearchModeExact, `log("Hello, World!")`, 1, 10)
		assert.Equal(t, int64(1), total)
		assert.Equal(t, []string{"example-file.js"}, files)
	})

	t.Run("regexp keeps escapes", func(t *testing.T) {
		total, files := search(t, indexer_module.SearchModeRegexp, `Hello,\sW\w+`, 1, 10)
		assert.Equal(t, int64(1), total)
		assert.Equal(t, []string{"example-file.js"}, files)
	})

	t.Run("zoekt syntax matches filenames", func(t *testing.T) {
		_, files := search(t, indexer_module.SearchModeZoekt, `file:avocado`, 1, 10)
		assert.Equal(t, []string{"avocado.md"}, files)
	})

	t.Run("pagination", func(t *testing.T) {
		// the pages of a result set have to be disjoint slices of it, walking
		// them must yield every result exactly once
		const pageSize = 2
		total, all := search(t, indexer_module.SearchModeExact, "is", 1, 100)
		require.Len(t, all, int(total))
		require.Greater(t, total, int64(pageSize), "the result set has to span more than one page")

		var paged []string
		for page := 1; (page-1)*pageSize < int(total); page++ {
			_, hits := search(t, indexer_module.SearchModeExact, "is", page, pageSize)
			paged = append(paged, hits...)
		}
		assert.Equal(t, all, paged)
	})
}
