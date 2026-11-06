// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	code_indexer "gitea.dev/modules/indexer/code"
	"gitea.dev/modules/indexer/code/gitgrep"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	t.Run("ScopedPages", func(t *testing.T) {
		for _, search := range []struct {
			url   string
			count int
		}{
			{"/explore/code?q=Description+repo:user2/repo1", 1},
			{"/explore/code?q=Description+repo:missing/missing", 0},
			{"/user2/-/code?q=Description", 1},
			{"/org3/-/code?q=Description", 0},
		} {
			resp := MakeRequest(t, NewRequest(t, http.MethodGet, search.url), http.StatusOK)
			assert.Equal(t, search.count, NewHTMLParser(t, resp.Body).doc.Find(".repo-search-result").Length(), search.url)
		}
	})

	doc := testSearch(t, "/user2/repo1/search?q=Description&page=1&search_mode=regexp&l=Go", []string{"README.md"})
	assert.Zero(t, doc.doc.Find(`form input[name="search_mode"]`).Length())
	languageLink, exists := doc.doc.Find("a.label").First().Attr("href")
	require.True(t, exists)
	languageURL, err := url.Parse(languageLink)
	require.NoError(t, err)
	assert.Equal(t, url.Values{"q": {"Description language:Markdown"}}, languageURL.Query())

	doc = testSearch(t, "/user2/repo1/search"+languageLink, []string{"README.md"})
	languageLink, exists = doc.doc.Find("a.primary.label").Attr("href")
	require.True(t, exists)
	languageURL, err = url.Parse(languageLink)
	require.NoError(t, err)
	assert.Equal(t, url.Values{"q": {"Description"}}, languageURL.Query())

	doc = testSearch(t, "/user2/repo1/search?q="+url.QueryEscape(`"unterminated`), []string{})
	assert.Contains(t, doc.doc.Find(".ui.error.message").Text(), "Invalid search query:")
	assert.NotContains(t, doc.doc.Find(".ui.list").Text(), "No matching results found.")

	t.Run("GitGrep", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Indexer.RepoIndexerEnabled, false)()
		testSearch(t, "/user2/repo1/search?q="+url.QueryEscape("/Descri.tion/"), []string{"README.md"})
	})

	defer test.MockVariableValue(&setting.Indexer.IncludePatterns, setting.IndexerGlobFromString("**.txt"))()
	defer test.MockVariableValue(&setting.Indexer.ExcludePatterns, setting.IndexerGlobFromString("**/y/**"))()

	repo, err = repo_model.GetRepositoryByOwnerAndName(t.Context(), "user2", "glob")
	require.NoError(t, err)
	files := []git.FastImportFile{
		{Path: "a.txt", Content: "file1 loren ipsum\n\n\n\nlastMatch\n"},
		{Path: "aaa.doc", Content: "file2 loren ipsum\n"},
		{Path: "x/b.txt", Content: "file3\n"},
		{Path: "x/y/a.txt", Content: "file4\n"},
		{Path: "x/y/z/a.txt", Content: "file5\n"},
	}
	for i := range gitgrep.MaxResultLimit + 1 {
		files = append(files, git.FastImportFile{Path: "cap" + strconv.Itoa(i) + ".txt", Content: "capped"})
	}
	require.NoError(t, git.ForceFastImport(t.Context(), repo.CodeStorageRepo(), []git.FastImportCommit{{Ref: "refs/heads/master", Files: files}}))
	code_indexer.UpdateRepoIndexer(repo)

	testSearch(t, "/user2/glob/search?q=loren&page=1", []string{"a.txt"})
	testSearch(t, "/user2/glob/search?q=loren+ipsum&page=1", []string{"a.txt"})
	doc = testSearch(t, "/user2/glob/search?q=loren+lastMatch", []string{"a.txt"})
	assert.Equal(t, 2, doc.doc.Find(".file-code").Length())
	assert.Equal(t, "5", doc.doc.Find(`.lines-num a[href$="#L5"]`).Text())
	assert.Zero(t, testSearch(t, "/user2/glob/search?q=a.txt", []string{"a.txt"}).doc.Find(".file-code").Length())
	testSearch(t, "/user2/glob/search?q=file3&page=1", []string{"x/b.txt"})
	testSearch(t, "/user2/glob/search?q="+url.QueryEscape("file3 path:x"), []string{"x/b.txt"})
	testSearch(t, "/user2/glob/search?q=file4&page=1&t=match", []string{})
	testSearch(t, "/user2/glob/search?q=file5&page=1&t=match", []string{})

	t.Run("GitGrepIncomplete", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Indexer.RepoIndexerEnabled, false)()
		resp := MakeRequest(t, NewRequest(t, http.MethodGet, "/user2/glob/search?q=capped"), http.StatusOK)
		assert.Contains(t, NewHTMLParser(t, resp.Body).doc.Find(".ui.warning.message").Text(), "Search stopped early")
		doc := testSearch(t, "/user2/glob/search?q=loren+lastMatch", []string{"a.txt"})
		assert.Equal(t, 2, doc.doc.Find(".file-code").Length())
		assert.Equal(t, "5", doc.doc.Find(`.lines-num a[href$="#L5"]`).Text())
		assert.Zero(t, testSearch(t, "/user2/glob/search?q=a.txt", []string{"a.txt"}).doc.Find(".file-code").Length())
	})

	defer test.MockVariableValue(&setting.UI.RepoSearchPagingNum, 1)()
	response := MakeRequest(t, NewRequest(t, http.MethodGet, "/user2/glob/search?q=file&search_mode=regexp&l=Go"), http.StatusOK)
	doc = NewHTMLParser(t, response.Body)
	paginationLinks := doc.doc.Find(".pagination a[href]")
	require.Positive(t, paginationLinks.Length())
	paginationLinks.Each(func(_index int, link *goquery.Selection) {
		paginationURL, err := url.Parse(link.AttrOr("href", ""))
		require.NoError(t, err)
		assert.False(t, paginationURL.Query().Has("search_mode"))
		assert.False(t, paginationURL.Query().Has("l"))
		assert.Equal(t, "file", paginationURL.Query().Get("q"))
	})
}

func testSearch(t *testing.T, url string, expected []string) *HTMLDoc {
	t.Helper()
	req := NewRequest(t, "GET", url)
	resp := MakeRequest(t, req, http.StatusOK)
	doc := NewHTMLParser(t, resp.Body)
	filenames := resultFilenames(doc)
	assert.Equal(t, expected, filenames, "url=%s", url)
	return doc
}
