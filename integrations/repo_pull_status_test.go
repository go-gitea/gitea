// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"path"
	"strings"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/sdk/gitea"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
)

var (
	statesIcons = map[models.CommitStatusState]string{
		models.CommitStatusPending: "circle icon yellow",
		models.CommitStatusSuccess: "check icon green",
		models.CommitStatusError:   "warning icon red",
		models.CommitStatusFailure: "remove icon red",
		models.CommitStatusWarning: "warning sign icon yellow",
	}
)

func TestRepoPullsWithStatus(t *testing.T) {
	prepareTestEnv(t)

	session := loginUser(t, "user2")

	var size = 5
	// create some pulls
	for i := 0; i < size; i++ {
		testEditFileToNewBranchAndSendPull(t, session, "user2", "repo16", "master", fmt.Sprintf("test%d", i), "readme.md")
	}

	// look for repo's pulls page
	req := NewRequest(t, "GET", "/user2/repo16/pulls")
	resp := session.MakeRequest(t, req, http.StatusOK)
	doc := NewHTMLParser(t, resp.Body)

	var indexes = make([]string, 0, size)
	doc.doc.Find("li.item").Each(func(idx int, s *goquery.Selection) {
		indexes = append(indexes, strings.TrimLeft(s.Find("div").Eq(1).Text(), "#"))
	})

	indexes = indexes[:5]

	var status = make([]models.CommitStatusState, len(indexes))
	for i := 0; i < len(indexes); i++ {
		switch i {
		case 0:
			status[i] = models.CommitStatusPending
		case 1:
			status[i] = models.CommitStatusSuccess
		case 2:
			status[i] = models.CommitStatusError
		case 3:
			status[i] = models.CommitStatusFailure
		case 4:
			status[i] = models.CommitStatusWarning
		default:
			status[i] = models.CommitStatusSuccess
		}
	}

	for i, index := range indexes {
		// Request repository commits page
		req = NewRequestf(t, "GET", "/user2/repo16/pulls/%s/commits", index)
		resp = session.MakeRequest(t, req, http.StatusOK)
		doc = NewHTMLParser(t, resp.Body)

		// Get first commit URL
		commitURL, exists := doc.doc.Find("#commits-table tbody tr td.sha a").Last().Attr("href")
		assert.True(t, exists)
		assert.NotEmpty(t, commitURL)

		commitID := path.Base(commitURL)
		// Call API to add status for commit
		req = NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo16/statuses/"+commitID,
			api.CreateStatusOption{
				State:       api.StatusState(status[i]),
				TargetURL:   "http://test.ci/",
				Description: "",
				Context:     "testci",
			},
		)
		session.MakeRequest(t, req, http.StatusCreated)

		req = NewRequestf(t, "GET", "/user2/repo16/pulls/%s/commits", index)
		resp = session.MakeRequest(t, req, http.StatusOK)
		doc = NewHTMLParser(t, resp.Body)

		commitURL, exists = doc.doc.Find("#commits-table tbody tr td.sha a").Last().Attr("href")
		assert.True(t, exists)
		assert.NotEmpty(t, commitURL)
		assert.EqualValues(t, commitID, path.Base(commitURL))

		cls, ok := doc.doc.Find("#commits-table tbody tr td.message i.commit-status").Last().Attr("class")
		assert.True(t, ok)
		assert.EqualValues(t, "commit-status "+statesIcons[status[i]], cls)
	}

	req = NewRequest(t, "GET", "/user2/repo16/pulls")
	resp = session.MakeRequest(t, req, http.StatusOK)
	doc = NewHTMLParser(t, resp.Body)

	doc.doc.Find("li.item").Each(func(i int, s *goquery.Selection) {
		cls, ok := s.Find("i.commit-status").Attr("class")
		assert.True(t, ok)
		assert.EqualValues(t, "commit-status "+statesIcons[status[i]], cls)
	})

	req = NewRequest(t, "GET", "/pulls?type=all&repo=16&sort=&state=open")
	resp = session.MakeRequest(t, req, http.StatusOK)
	doc = NewHTMLParser(t, resp.Body)

	fmt.Println(string(resp.Body))

	doc.doc.Find("li.item").Each(func(i int, s *goquery.Selection) {
		cls, ok := s.Find("i.commit-status").Attr("class")
		assert.True(t, ok)
		assert.EqualValues(t, "commit-status "+statesIcons[status[i]], cls)
	})
}
