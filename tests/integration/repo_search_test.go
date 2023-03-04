// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration

import (
	"net/http"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	code_indexer "code.gitea.io/gitea/modules/indexer/code"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
)

func resultFilenames(t testing.TB, doc *HTMLDoc) []string {
	filenameSelections := doc.doc.Find(".repository.search").Find(".repo-search-result").Find(".header").Find("span.file")
	result := make([]string, filenameSelections.Length())
	filenameSelections.Each(func(i int, selection *goquery.Selection) {
		result[i] = selection.Text()
	})
	return result
}

func TestSearchRepo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo, err := repo_model.GetRepositoryByOwnerAndName("user2", "repo1")
	assert.NoError(t, err)

	executeIndexer(t, repo, code_indexer.UpdateRepoIndexer)

	testSearch(t, "/user2/repo1/search?q=Description&page=1", []string{"README.md"})

	setting.Indexer.IncludePatterns = setting.IndexerGlobFromString("**.txt")
	setting.Indexer.ExcludePatterns = setting.IndexerGlobFromString("**/y/**")

	repo, err = repo_model.GetRepositoryByOwnerAndName("user2", "glob")
	assert.NoError(t, err)

	executeIndexer(t, repo, code_indexer.UpdateRepoIndexer)

	testSearch(t, "/user2/glob/search?q=loren&page=1", []string{"a.txt"})
	testSearch(t, "/user2/glob/search?q=file3&page=1", []string{"x/b.txt"})
	testSearch(t, "/user2/glob/search?q=file4&page=1", []string{})
	testSearch(t, "/user2/glob/search?q=file5&page=1", []string{})
}

func testSearch(t *testing.T, url string, expected []string) {
	req := NewRequestf(t, "GET", url)
	resp := MakeRequest(t, req, http.StatusOK)

	filenames := resultFilenames(t, NewHTMLParser(t, resp.Body))
	assert.EqualValues(t, expected, filenames)
}

func executeIndexer(t *testing.T, repo *repo_model.Repository, op func(*repo_model.Repository)) {
	op(repo)
}
