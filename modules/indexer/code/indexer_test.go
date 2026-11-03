// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package code

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/git"
	indexer_module "gitea.dev/modules/indexer"
	"gitea.dev/modules/indexer/code/bleve"
	"gitea.dev/modules/indexer/code/elasticsearch"
	"gitea.dev/modules/indexer/code/internal"
	"gitea.dev/modules/test"
	"gitea.dev/modules/util"

	_ "gitea.dev/models"
	_ "gitea.dev/models/actions"
	_ "gitea.dev/models/activities"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type codeSearchResult struct {
	Filename string
	Content  string
}

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

const unicodeSearchContent = `Поставщик
日本語のテキスト
हिन्दी
ภาษาไทย
İSTANBUL
<em>literal</em>
abc bcd cde def
`

func testIndexer(name string, t *testing.T, indexer internal.Indexer) {
	t.Run(name, func(t *testing.T) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 62})
		require.NoError(t, git.ForceFastImport(t.Context(), repo, []git.FastImportCommit{{
			Ref: "refs/heads/master",
			Files: []git.FastImportFile{
				{Path: "avocado.md", Content: "# repo1\n\npineaple pie of cucumber juice"},
				{Path: "cucumber.md", Content: "Salad is good for your health"},
				{Path: "example-file.js", Content: "console.log(\"Hello, World!\")"},
				{Path: "ham.md", Content: "This is also not cheese"},
				{Path: "potato/ham.md", Content: "This is not cheese"},
				{Path: "unicode.txt", Content: unicodeSearchContent},
			},
		}}))
		assert.NoError(t, setupRepositoryIndexes(t.Context(), indexer))
		// Wait for the index to catch up: ES/OpenSearch make writes visible
		// only after a refresh (default interval: 1s). Bleve is synchronous
		// and passes on the first iteration.
		require.Eventually(t, func() bool {
			total, _, _, err := indexer.Search(t.Context(), &internal.SearchOptions{
				Keyword:   "Description",
				Paginator: &db.ListOptions{Page: 1, PageSize: 1},
			})
			return err == nil && total > 0
		}, 10*time.Second, 100*time.Millisecond, "index did not become searchable")

		keywords := []struct {
			RepoIDs    []int64
			Keyword    string
			Langs      int
			SearchMode indexer_module.SearchModeType
			Results    []codeSearchResult
			Match      string
		}{
			// Search for an exact match on the contents of a file
			// This scenario yields a single result (the file README.md on the repo '1')
			{
				RepoIDs: nil,
				Keyword: "Description",
				Langs:   1,
				Results: []codeSearchResult{
					{
						Filename: "README.md",
						Content:  "# repo1\n\nDescription for repo1",
					},
				},
			},
			// Search for an exact match on the contents of a file within the repo '2'.
			// This scenario yields no results
			{
				RepoIDs: []int64{2},
				Keyword: "Description",
				Langs:   0,
			},
			// Search for an exact match on the contents of a file
			// This scenario yields a single result (the file README.md on the repo '1')
			{
				RepoIDs: nil,
				Keyword: "repo1",
				Langs:   1,
				Results: []codeSearchResult{
					{
						Filename: "README.md",
						Content:  "# repo1\n\nDescription for repo1",
					},
					{
						Filename: "avocado.md",
						Content:  "# repo1\n\npineaple pie of cucumber juice",
					},
				},
			},
			// Search for an exact match on the contents of a file within the repo '2'.
			// This scenario yields no results
			{
				RepoIDs: []int64{2},
				Keyword: "repo1",
				Langs:   0,
			},
			// Search for a non-existing term.
			// This scenario yields no results
			{
				RepoIDs: nil,
				Keyword: "non-exist",
				Langs:   0,
			},
			// Search for an exact match on the contents of a file within the repo '62'.
			// This scenario yields a single result (the file avocado.md on the repo '62')
			{
				RepoIDs: []int64{62},
				Keyword: "pineaple",
				Langs:   1,
				Results: []codeSearchResult{
					{
						Filename: "avocado.md",
						Content:  "# repo1\n\npineaple pie of cucumber juice",
					},
				},
			},
			// Search for an exact match on the filename within the repo '62'.
			// This scenario yields a single result (the file avocado.md on the repo '62')
			{
				RepoIDs: []int64{62},
				Keyword: "avocado.md",
				Langs:   1,
				Results: []codeSearchResult{
					{
						Filename: "avocado.md",
						Content:  "# repo1\n\npineaple pie of cucumber juice",
					},
				},
			},
			// Search for an partial match on the filename within the repo '62'.
			// This scenario yields a single result (the file avocado.md on the repo '62')
			{
				RepoIDs: []int64{62},
				Keyword: "avo",
				Langs:   1,
				Results: []codeSearchResult{
					{
						Filename: "avocado.md",
						Content:  "# repo1\n\npineaple pie of cucumber juice",
					},
				},
			},
			// Search for matches on both the contents and the filenames within the repo '62'.
			// This scenario yields two results: the first result is based on the file (cucumber.md) while the second is based on the contents
			{
				RepoIDs: []int64{62},
				Keyword: "cucumber",
				Langs:   1,
				Results: []codeSearchResult{
					{
						Filename: "cucumber.md",
						Content:  "Salad is good for your health",
					},
					{
						Filename: "avocado.md",
						Content:  "# repo1\n\npineaple pie of cucumber juice",
					},
				},
			},
			// Search for matches on the filenames within the repo '62'.
			// This scenario yields two results (both are based on filename, the first one is an exact match)
			{
				RepoIDs: []int64{62},
				Keyword: "ham",
				Langs:   1,
				Results: []codeSearchResult{
					{
						Filename: "ham.md",
						Content:  "This is also not cheese",
					},
					{
						Filename: "potato/ham.md",
						Content:  "This is not cheese",
					},
				},
			},
			// Search for a match on the full path of a file within the repo '62'.
			{
				RepoIDs: []int64{62},
				Keyword: "potato/ham",
				Langs:   1,
				Results: []codeSearchResult{
					{
						Filename: "potato/ham.md",
						Content:  "This is not cheese",
					},
				},
			},
			// Search for matches on the contents of files within the repo '62'.
			// This scenario yields two results (both are based on contents, the first one is an exact match where as the second is a words match)
			{
				RepoIDs: []int64{62},
				Keyword: "This is not cheese",
				Langs:   1,
				Results: []codeSearchResult{
					{
						Filename: "potato/ham.md",
						Content:  "This is not cheese",
					},
					{
						Filename: "ham.md",
						Content:  "This is also not cheese",
					},
				},
			},
			// Search for matches on the contents of files regardless of case.
			{
				RepoIDs: nil,
				Keyword: "dESCRIPTION",
				Langs:   1,
				Results: []codeSearchResult{
					{
						Filename: "README.md",
						Content:  "# repo1\n\nDescription for repo1",
					},
				},
			},
			// Search for an exact match on the filename within the repo '62' (case-insensitive).
			// This scenario yields a single result (the file avocado.md on the repo '62')
			{
				RepoIDs: []int64{62},
				Keyword: "AVOCADO.MD",
				Langs:   1,
				Results: []codeSearchResult{
					{
						Filename: "avocado.md",
						Content:  "# repo1\n\npineaple pie of cucumber juice",
					},
				},
			},
			// Search for matches on the contents of files when the criteria are an expression.
			{
				RepoIDs: []int64{62},
				Keyword: "console.log",
				Langs:   1,
				Results: []codeSearchResult{
					{
						Filename: "example-file.js",
						Content:  "console.log(\"Hello, World!\")",
					},
				},
			},
			// Search for matches on the contents of files when the criteria are parts of an expression.
			{
				RepoIDs: []int64{62},
				Keyword: "log",
				Langs:   1,
				Results: []codeSearchResult{
					{
						Filename: "example-file.js",
						Content:  "console.log(\"Hello, World!\")",
					},
				},
			},
			{RepoIDs: []int64{62}, Keyword: "ставщ", Langs: 1, SearchMode: indexer_module.SearchModeExact, Results: []codeSearchResult{{Filename: "unicode.txt", Content: unicodeSearchContent}}},
			{RepoIDs: []int64{62}, Keyword: "テキスト", Langs: 1, Results: []codeSearchResult{{Filename: "unicode.txt", Content: unicodeSearchContent}}},
			{RepoIDs: []int64{62}, Keyword: "हिन्दी", Langs: 1, SearchMode: indexer_module.SearchModeExact, Results: []codeSearchResult{{Filename: "unicode.txt", Content: unicodeSearchContent}}},
			{RepoIDs: []int64{62}, Keyword: "iSTANBUL", Langs: 1, SearchMode: indexer_module.SearchModeExact, Match: "İSTANBUL", Results: []codeSearchResult{{Filename: "unicode.txt", Content: unicodeSearchContent}}},
			{RepoIDs: []int64{62}, Keyword: "literal", Langs: 1, Match: "literal", Results: []codeSearchResult{{Filename: "unicode.txt", Content: unicodeSearchContent}}},
			{RepoIDs: []int64{62}, Keyword: "日本 ภาษา", Langs: 1, Match: "日本", Results: []codeSearchResult{{Filename: "unicode.txt", Content: unicodeSearchContent}}},
			{RepoIDs: []int64{62}, Keyword: "abcdef", SearchMode: indexer_module.SearchModeExact},
		}

		for _, kw := range keywords {
			t.Run(kw.Keyword, func(t *testing.T) {
				total, res, langs, err := indexer.Search(t.Context(), &internal.SearchOptions{
					RepoIDs:    kw.RepoIDs,
					Keyword:    kw.Keyword,
					SearchMode: util.IfZero(kw.SearchMode, indexer_module.SearchModeWords),
					Paginator: &db.ListOptions{
						Page:     1,
						PageSize: 10,
					},
				})
				require.NoError(t, err)
				require.Len(t, langs, kw.Langs)
				require.EqualValues(t, len(kw.Results), total)
				require.Len(t, res, len(kw.Results))

				hits := make([]codeSearchResult, 0, len(res))

				for _, hit := range res {
					if kw.Match != "" {
						require.Equal(t, kw.Match, hit.Content[hit.StartIndex:hit.EndIndex])
					}
					hits = append(hits, codeSearchResult{
						Filename: hit.Filename,
						Content:  hit.Content,
					})
				}

				lastIndex := -1

				for _, expected := range kw.Results {
					index := slices.Index(hits, expected)
					if index == -1 {
						assert.Failf(t, "Result not found", "Expected %v in %v", expected, hits)
					} else if lastIndex > index {
						assert.Failf(t, "Result is out of order", "The order of %v within %v is wrong", expected, hits)
					} else {
						lastIndex = index
					}
				}
			})
		}

		assert.NoError(t, tearDownRepositoryIndexes(t.Context(), indexer))
	})
}

func TestBleveIndexAndSearch(t *testing.T) {
	unittest.PrepareTestEnv(t)
	dir := t.TempDir()

	idx := bleve.NewIndexer(dir)
	defer idx.Close()

	_, err := idx.Init(t.Context())
	require.NoError(t, err)

	testIndexer("bleve", t, idx)
}

func TestESIndexAndSearch(t *testing.T) {
	unittest.PrepareTestEnv(t)

	u := test.ExternalServiceHTTP(t, "TEST_ELASTICSEARCH_URL", "http://elasticsearch:9200")

	indexer := elasticsearch.NewIndexer(u, fmt.Sprintf("test_code_indexer_%d", time.Now().UnixNano()))
	if _, err := indexer.Init(t.Context()); err != nil {
		if indexer != nil {
			indexer.Close()
		}
		require.NoError(t, err, "Unable to init ES indexer")
	}

	defer indexer.Close()

	testIndexer("elastic_search", t, indexer)
}

func setupRepositoryIndexes(ctx context.Context, indexer internal.Indexer) error {
	for _, repoID := range repositoriesToSearch() {
		if err := index(ctx, indexer, repoID); err != nil {
			return err
		}
	}
	return nil
}

func tearDownRepositoryIndexes(ctx context.Context, indexer internal.Indexer) error {
	for _, repoID := range repositoriesToSearch() {
		if err := indexer.Delete(ctx, repoID); err != nil {
			return err
		}
	}
	return nil
}

func repositoriesToSearch() []int64 {
	return []int64{1, 62}
}
