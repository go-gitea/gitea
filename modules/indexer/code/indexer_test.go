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

	_ "code.gitea.io/gitea/models"
	_ "code.gitea.io/gitea/models/actions"
	_ "code.gitea.io/gitea/models/activities"

	"github.com/stretchr/testify/assert"

	_ "github.com/mattn/go-sqlite3"
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
			{
				RepoIDs: []int64{2},
				Keyword: "Description",
				Langs:   0,
			},
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
			{
				RepoIDs: []int64{2},
				Keyword: "repo1",
				Langs:   0,
			},
			{
				RepoIDs: nil,
				Keyword: "non-exist",
				Langs:   0,
			},
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

	dir := t.TempDir()

	idx := bleve.NewIndexer(dir)
	_, err := idx.Init(context.Background())
	if err != nil {
		if idx != nil {
			idx.Close()
		}
		assert.FailNow(t, "Unable to create bleve indexer Error: %v", err)
	}
	defer idx.Close()

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
