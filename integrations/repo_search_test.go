// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	code_indexer "code.gitea.io/gitea/modules/indexer/code"
	"code.gitea.io/gitea/modules/setting"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
)

func resultFilenames(t testing.TB, doc *HTMLDoc) []string {
	resultsSelection := doc.doc.Find(".repository.search")
	assert.EqualValues(t, 1, resultsSelection.Length(),
		"Invalid template (repo search template has changed?)")
	filenameSelections := resultsSelection.Find(".repo-search-result").Find(".header").Find("span.file")
	result := make([]string, filenameSelections.Length())
	filenameSelections.Each(func(i int, selection *goquery.Selection) {
		result[i] = selection.Text()
	})
	return result
}

func TestSearchRepo(t *testing.T) {
	defer prepareTestEnv(t)()

	repo, err := models.GetRepositoryByOwnerAndName("user2", "repo1")
	assert.NoError(t, err)

	executeIndexer(t, repo, code_indexer.UpdateRepoIndexer)

	testSearch(t, "/user2/repo1/search?q=Description&page=1", []string{"README.md"})

	setting.Indexer.IncludePatterns = setting.IndexerGlobFromString("**.txt")
	setting.Indexer.ExcludePatterns = setting.IndexerGlobFromString("**/y/**")

	repo, err = models.GetRepositoryByOwnerAndName("user2", "glob")
	assert.NoError(t, err)

	executeIndexer(t, repo, code_indexer.DeleteRepoFromIndexer)
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

func executeIndexer(t *testing.T, repo *models.Repository, op func(*models.Repository, ...chan<- error)) {
	waiter := make(chan error, 1)
	op(repo, waiter)

	select {
	case err := <-waiter:
		assert.NoError(t, err)
	case <-time.After(1 * time.Minute):
		assert.Fail(t, "Repository indexer took too long")
	}
}
