// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"log"
	"net/http"
	"testing"
	"time"

	"code.gitea.io/gitea/models"

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
	prepareTestEnv(t)

	repo, err := models.GetRepositoryByOwnerAndName("user2", "repo1")
	assert.NoError(t, err)

	models.UpdateRepoIndexer(repo)

	log.Printf("Waiting for indexing\n")

	i := 0
	for {
		if repo.IndexerStatus != nil && len(repo.IndexerStatus.CommitSha) != 0 && i <= 60 {
			break
		}
		time.Sleep(1 * time.Second)
		i++
	}
	log.Printf("Indexing took: %d s\n", i)

	req := NewRequestf(t, "GET", "/user2/repo1/search?q=Description&page=1")
	resp := MakeRequest(t, req, http.StatusOK)

	filenames := resultFilenames(t, NewHTMLParser(t, resp.Body))
	assert.EqualValues(t, []string{"README.md"}, filenames)
}
