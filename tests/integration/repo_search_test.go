// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	code_indexer "code.gitea.io/gitea/modules/indexer/code"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/tests"

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

	repo, err := repo_model.GetRepositoryByOwnerAndName(db.DefaultContext, "user2", "repo1")
	assert.NoError(t, err)

	code_indexer.UpdateRepoIndexer(repo)

	testSearch(t, "/user2/repo1/search?q=Description&page=1", []string{"README.md"})

	setting.Indexer.IncludePatterns = setting.IndexerGlobFromString("**.txt")
	setting.Indexer.ExcludePatterns = setting.IndexerGlobFromString("**/y/**")

	repo, err = repo_model.GetRepositoryByOwnerAndName(db.DefaultContext, "user2", "glob")
	assert.NoError(t, err)

	code_indexer.UpdateRepoIndexer(repo)

	testSearch(t, "/user2/glob/search?q=loren&page=1", []string{"a.txt"})
	testSearch(t, "/user2/glob/search?q=loren&page=1&t=match", []string{"a.txt"})
	testSearch(t, "/user2/glob/search?q=file3&page=1", []string{"x/b.txt", "a.txt"})
	testSearch(t, "/user2/glob/search?q=file3&page=1&t=match", []string{"x/b.txt", "a.txt"})
	testSearch(t, "/user2/glob/search?q=file4&page=1&t=match", []string{"x/b.txt", "a.txt"})
	testSearch(t, "/user2/glob/search?q=file5&page=1&t=match", []string{"x/b.txt", "a.txt"})
}

func testSearch(t *testing.T, url string, expected []string) {
	req := NewRequest(t, "GET", url)
	resp := MakeRequest(t, req, http.StatusOK)

	filenames := resultFilenames(NewHTMLParser(t, resp.Body))
	assert.EqualValues(t, expected, filenames)
}
