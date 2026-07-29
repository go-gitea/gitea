// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	repo_model "gitea.dev/models/repo"
	code_indexer "gitea.dev/modules/indexer/code"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
)

func resultFilenames(doc *HTMLDoc) []string {
	filenameSelections := doc.doc.Find(".repository.search").Find(".repo-search-result").Find(".header").Find("span.file")
	result := make([]string, filenameSelections.Length())
	filenameSelections.Each(func(i int, selection *goquery.Selection) {
		result[i] = selection.Text()
	})
	return result
}

func TestSearchRepo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo, err := repo_model.GetRepositoryByOwnerAndName(t.Context(), "user2", "repo1")
	assert.NoError(t, err)

	code_indexer.UpdateRepoIndexer(repo)

	testSearch(t, "/user2/repo1/search?q=Description&page=1", []string{"README.md"})

	defer test.MockVariableValue(&setting.Indexer.IncludePatterns, setting.IndexerGlobFromString("**.txt"))()
	defer test.MockVariableValue(&setting.Indexer.ExcludePatterns, setting.IndexerGlobFromString("**/y/**"))()

	repo, err = repo_model.GetRepositoryByOwnerAndName(t.Context(), "user2", "glob")
	assert.NoError(t, err)

	code_indexer.UpdateRepoIndexer(repo)

	testSearch(t, "/user2/glob/search?q=loren&page=1", []string{"a.txt"})
	testSearch(t, "/user2/glob/search?q=loren&page=1&t=match", []string{"a.txt"})
	// a.txt contains "file1", x/b.txt contains "file3" verbatim, so "file3" only
	// matches x/b.txt. Before #37221 was fixed, the bleve tokenizer dropped digit
	// runs entirely, collapsing "file1" and "file3" to the same "file" token, so
	// this assertion used to (incidentally, not by design) also match a.txt.
	testSearch(t, "/user2/glob/search?q=file3&page=1", []string{"x/b.txt"})
	testSearch(t, "/user2/glob/search?q=file3&page=1&t=match", []string{"x/b.txt"})
	// "file4"/"file5" only appear in x/y/a.txt and x/y/z/a.txt, both excluded by the
	// "**/y/**" ExcludePatterns above — so these correctly match nothing now that
	// digit-aware tokenization (#37221) makes "file4"/"file5" distinct search terms
	// instead of everything collapsing to a bare "file" token. This is a stronger
	// check of ExcludePatterns than before: previously these assertions passed only
	// because of the digit-dropping bug, not because exclusion was actually verified.
	testSearch(t, "/user2/glob/search?q=file4&page=1&t=match", []string{})
	testSearch(t, "/user2/glob/search?q=file5&page=1&t=match", []string{})
}

func testSearch(t *testing.T, url string, expected []string) {
	req := NewRequest(t, "GET", url)
	resp := MakeRequest(t, req, http.StatusOK)

	filenames := resultFilenames(NewHTMLParser(t, resp.Body))
	assert.Equal(t, expected, filenames)
}
