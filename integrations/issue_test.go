// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"path"
	"strconv"
	"strings"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
)

func getIssuesSelection(htmlDoc *HTMLDoc) *goquery.Selection {
	return htmlDoc.doc.Find(".issue.list").Find("li").Find(".title")
}

func getIssue(t *testing.T, repoID int64, issueSelection *goquery.Selection) *models.Issue {
	href, exists := issueSelection.Attr("href")
	assert.True(t, exists)
	indexStr := href[strings.LastIndexByte(href, '/')+1:]
	index, err := strconv.Atoi(indexStr)
	assert.NoError(t, err, "Invalid issue href: %s", href)
	return models.AssertExistsAndLoadBean(t, &models.Issue{RepoID: repoID, Index: int64(index)}).(*models.Issue)
}

func TestNoLoginViewIssues(t *testing.T) {
	prepareTestEnv(t)

	req := NewRequest(t, "GET", "/user2/repo1/issues")
	MakeRequest(t, req, http.StatusOK)
}

func TestNoLoginViewIssuesSortByType(t *testing.T) {
	prepareTestEnv(t)

	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 1}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	repo.Owner = models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, user.Name)
	req := NewRequest(t, "GET", repo.RelLink()+"/issues?type=created_by")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	issuesSelection := getIssuesSelection(htmlDoc)
	expectedNumIssues := models.GetCount(t,
		&models.Issue{RepoID: repo.ID, PosterID: user.ID},
		models.Cond("is_closed=?", false),
		models.Cond("is_pull=?", false),
	)
	if expectedNumIssues > setting.UI.IssuePagingNum {
		expectedNumIssues = setting.UI.IssuePagingNum
	}
	assert.EqualValues(t, expectedNumIssues, issuesSelection.Length())

	issuesSelection.Each(func(_ int, selection *goquery.Selection) {
		issue := getIssue(t, repo.ID, selection)
		assert.EqualValues(t, user.ID, issue.PosterID)
	})
}

func TestNoLoginViewIssue(t *testing.T) {
	prepareTestEnv(t)

	req := NewRequest(t, "GET", "/user2/repo1/issues/1")
	MakeRequest(t, req, http.StatusOK)
}

func testNewIssue(t *testing.T, session *TestSession, user, repo, title string) {

	req := NewRequest(t, "GET", path.Join(user, repo, "issues", "new"))
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find("form.ui.form").Attr("action")
	assert.True(t, exists, "The template has changed")
	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
		"title": title,
	})
	resp = session.MakeRequest(t, req, http.StatusFound)

	req = NewRequest(t, "GET", RedirectURL(t, resp))
	resp = session.MakeRequest(t, req, http.StatusOK)
}

func TestNewIssue(t *testing.T) {
	prepareTestEnv(t)
	session := loginUser(t, "user2")
	testNewIssue(t, session, "user2", "repo1", "Title")
}
