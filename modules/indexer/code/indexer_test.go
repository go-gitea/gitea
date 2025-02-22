// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package code

import (
	"context"
	"os"
	"slices"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/indexer/code/bleve"
	"code.gitea.io/gitea/modules/indexer/code/elasticsearch"
	"code.gitea.io/gitea/modules/indexer/code/internal"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	_ "code.gitea.io/gitea/models"
	_ "code.gitea.io/gitea/models/actions"
	_ "code.gitea.io/gitea/models/activities"

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

func testIndexer(name string, t *testing.T, indexer internal.Indexer) {
	t.Run(name, func(t *testing.T) {
		assert.NoError(t, setupRepositoryIndexes(git.DefaultContext, indexer))

		keywords := []struct {
			RepoIDs []int64
			Keyword string
			Langs   int
			Results []codeSearchResult
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
			// This scenario yields two results: the first result is baed on the file (cucumber.md) while the second is based on the contents
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
			// Search for matches on the contents of files within the repo '62'.
			// This scenario yields two results (both are based on contents, the first one is an exact match where as the second is a 'fuzzy' one)
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
			// Search for an exact match on the filename within the repo '62' (case insenstive).
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
			// Search for matches on the contents of files when the criteria is a expression.
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
			// Search for matches on the contents of files when the criteria is part of a expression.
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
		}

		for _, kw := range keywords {
			t.Run(kw.Keyword, func(t *testing.T) {
				total, res, langs, err := indexer.Search(context.TODO(), &internal.SearchOptions{
					RepoIDs: kw.RepoIDs,
					Keyword: kw.Keyword,
					Paginator: &db.ListOptions{
						Page:     1,
						PageSize: 10,
					},
					IsKeywordFuzzy: true,
				})
				assert.NoError(t, err)
				assert.Len(t, langs, kw.Langs)

				hits := make([]codeSearchResult, 0, len(res))

				if total > 0 {
					assert.NotEmpty(t, kw.Results, "The given scenario does not provide any expected results")
				}

				for _, hit := range res {
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

		assert.NoError(t, tearDownRepositoryIndexes(indexer))
	})
}

func TestBleveIndexAndSearch(t *testing.T) {
	unittest.PrepareTestEnv(t)
	defer test.MockVariableValue(&setting.Indexer.TypeBleveMaxFuzzniess, 2)()
	dir := t.TempDir()

	idx := bleve.NewIndexer(dir)
	defer idx.Close()

	_, err := idx.Init(context.Background())
	require.NoError(t, err)

	testIndexer("beleve", t, idx)
}

func TestESIndexAndSearch(t *testing.T) {
	unittest.PrepareTestEnv(t)

	u := os.Getenv("TEST_INDEXER_CODE_ES_URL")
	if u == "" {
		t.SkipNow()
		return
	}

	indexer := elasticsearch.NewIndexer(u, "gitea_codes")
	if _, err := indexer.Init(context.Background()); err != nil {
		if indexer != nil {
			indexer.Close()
		}
		assert.FailNow(t, "Unable to init ES indexer Error: %v", err)
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

func tearDownRepositoryIndexes(indexer internal.Indexer) error {
	for _, repoID := range repositoriesToSearch() {
		if err := indexer.Delete(context.Background(), repoID); err != nil {
			return err
		}
	}
	return nil
}

func repositoriesToSearch() []int64 {
	return []int64{1, 62}
}
