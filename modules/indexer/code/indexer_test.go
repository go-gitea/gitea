// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package code

import (
	"context"
	"os"
	"slices"
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	indexer_module "code.gitea.io/gitea/modules/indexer"
	"code.gitea.io/gitea/modules/indexer/code/bleve"
	"code.gitea.io/gitea/modules/indexer/code/elasticsearch"
	"code.gitea.io/gitea/modules/indexer/code/internal"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/util"

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
		assert.NoError(t, setupRepositoryIndexes(t.Context(), indexer))

		keywords := []struct {
			RepoIDs    []int64
			Keyword    string
			Langs      int
			SearchMode indexer_module.SearchModeType
			Results    []codeSearchResult
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
				RepoIDs:    nil,
				Keyword:    "dESCRIPTION",
				Langs:      1,
				SearchMode: indexer_module.SearchModeFuzzy,
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

		assert.NoError(t, tearDownRepositoryIndexes(t.Context(), indexer))
	})

	t.Run(name+"_archived_filter", func(t *testing.T) {
		assert.NoError(t, setupRepositoryIndexes(t.Context(), indexer))
		t.Cleanup(func() {
			assert.NoError(t, tearDownRepositoryIndexes(context.Background(), indexer))
		})

		repo1, err := repo_model.GetRepositoryByID(t.Context(), 1)
		require.NoError(t, err)
		repo62, err := repo_model.GetRepositoryByID(t.Context(), 62)
		require.NoError(t, err)

		originalRepo1Archived := repo1.IsArchived
		originalRepo62Archived := repo62.IsArchived
		t.Cleanup(func() {
			ctx := context.Background()
			require.NoError(t, repo_model.SetArchiveRepoState(ctx, repo1, originalRepo1Archived))
			require.NoError(t, repo_model.SetArchiveRepoState(ctx, repo62, originalRepo62Archived))
			require.NoError(t, repo_model.UpdateIndexerStatus(ctx, repo1, repo_model.RepoIndexerTypeCode, ""))
			require.NoError(t, repo_model.UpdateIndexerStatus(ctx, repo62, repo_model.RepoIndexerTypeCode, ""))
			require.NoError(t, index(ctx, indexer, repo1.ID))
			require.NoError(t, index(ctx, indexer, repo62.ID))
		})

		require.NoError(t, repo_model.SetArchiveRepoState(t.Context(), repo1, false))
		require.NoError(t, repo_model.SetArchiveRepoState(t.Context(), repo62, true))
		require.NoError(t, repo_model.UpdateIndexerStatus(t.Context(), repo1, repo_model.RepoIndexerTypeCode, ""))
		require.NoError(t, repo_model.UpdateIndexerStatus(t.Context(), repo62, repo_model.RepoIndexerTypeCode, ""))
		require.NoError(t, index(t.Context(), indexer, repo1.ID))
		require.NoError(t, index(t.Context(), indexer, repo62.ID))

		testCases := []struct {
			name      string
			keyword   string
			archived  optional.Option[bool]
			total     int64
			filenames []string
		}{
			{
				name:      "exclude_archived_repo_results",
				keyword:   "cucumber",
				archived:  optional.Some(false),
				total:     0,
				filenames: []string{},
			},
			{
				name:      "include_archived_repo_results",
				keyword:   "cucumber",
				archived:  optional.None[bool](),
				total:     2,
				filenames: []string{"cucumber.md", "avocado.md"},
			},
			{
				name:      "only_archived_repo_results",
				keyword:   "cucumber",
				archived:  optional.Some(true),
				total:     2,
				filenames: []string{"cucumber.md", "avocado.md"},
			},
			{
				name:      "exclude_keeps_non_archived_repo_results",
				keyword:   "Description",
				archived:  optional.Some(false),
				total:     1,
				filenames: []string{"README.md"},
			},
			{
				name:      "include_keeps_non_archived_repo_results",
				keyword:   "Description",
				archived:  optional.None[bool](),
				total:     1,
				filenames: []string{"README.md"},
			},
			{
				name:      "only_excludes_non_archived_repo_results",
				keyword:   "Description",
				archived:  optional.Some(true),
				total:     0,
				filenames: []string{},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				total, res, _, err := indexer.Search(t.Context(), &internal.SearchOptions{
					Keyword:    tc.keyword,
					SearchMode: indexer_module.SearchModeWords,
					Archived:   tc.archived,
					Paginator: &db.ListOptions{
						Page:     1,
						PageSize: 10,
					},
				})
				require.NoError(t, err)
				assert.Equal(t, tc.total, total)

				filenames := make([]string, 0, len(res))
				for _, hit := range res {
					filenames = append(filenames, hit.Filename)
				}
				assert.Equal(t, tc.filenames, filenames)
			})
		}
	})
}

func TestBleveIndexAndSearch(t *testing.T) {
	unittest.PrepareTestEnv(t)
	defer test.MockVariableValue(&setting.Indexer.TypeBleveMaxFuzzniess, 2)()
	dir := t.TempDir()

	idx := bleve.NewIndexer(dir)
	defer idx.Close()

	_, err := idx.Init(t.Context())
	require.NoError(t, err)

	testIndexer("bleve", t, idx)
}

func TestESIndexAndSearch(t *testing.T) {
	unittest.PrepareTestEnv(t)

	u := os.Getenv("TEST_INDEXER_CODE_ES_URL")
	if u == "" {
		t.SkipNow()
		return
	}

	indexer := elasticsearch.NewIndexer(u, "gitea_codes")
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
		repo, err := repo_model.GetRepositoryByID(ctx, repoID)
		if err != nil {
			return err
		}
		if err := repo_model.UpdateIndexerStatus(ctx, repo, repo_model.RepoIndexerTypeCode, ""); err != nil {
			return err
		}
	}
	return nil
}

func repositoriesToSearch() []int64 {
	return []int64{1, 62}
}
