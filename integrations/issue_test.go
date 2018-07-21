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
	"code.gitea.io/gitea/modules/test"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
)

func getIssuesSelection(t testing.TB, htmlDoc *HTMLDoc) *goquery.Selection {
	issueList := htmlDoc.doc.Find(".issue.list")
	assert.EqualValues(t, 1, issueList.Length())
	return issueList.Find("li").Find(".title")
}

func getIssue(t *testing.T, repoID int64, issueSelection *goquery.Selection) *models.Issue {
	href, exists := issueSelection.Attr("href")
	assert.True(t, exists)
	indexStr := href[strings.LastIndexByte(href, '/')+1:]
	index, err := strconv.Atoi(indexStr)
	assert.NoError(t, err, "Invalid issue href: %s", href)
	return models.AssertExistsAndLoadBean(t, &models.Issue{RepoID: repoID, Index: int64(index)}).(*models.Issue)
}

func assertMatch(t testing.TB, issue *models.Issue, keyword string) {
	matches := strings.Contains(strings.ToLower(issue.Title), keyword) ||
		strings.Contains(strings.ToLower(issue.Content), keyword)
	for _, comment := range issue.Comments {
		matches = matches || strings.Contains(
			strings.ToLower(comment.Content),
			keyword,
		)
	}
	assert.True(t, matches)
}

func TestNoLoginViewIssues(t *testing.T) {
	prepareTestEnv(t)

	req := NewRequest(t, "GET", "/user2/repo1/issues")
	MakeRequest(t, req, http.StatusOK)
}

func TestViewIssuesSortByType(t *testing.T) {
	prepareTestEnv(t)

	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 1}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)

	session := loginUser(t, user.Name)
	req := NewRequest(t, "GET", repo.RelLink()+"/issues?type=created_by")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	issuesSelection := getIssuesSelection(t, htmlDoc)
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

func TestViewIssuesKeyword(t *testing.T) {
	prepareTestEnv(t)

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)

	const keyword = "first"
	req := NewRequestf(t, "GET", "%s/issues?q=%s", repo.RelLink(), keyword)
	resp := MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	issuesSelection := getIssuesSelection(t, htmlDoc)
	assert.EqualValues(t, 1, issuesSelection.Length())
	issuesSelection.Each(func(_ int, selection *goquery.Selection) {
		issue := getIssue(t, repo.ID, selection)
		assert.False(t, issue.IsClosed)
		assert.False(t, issue.IsPull)
		assertMatch(t, issue, keyword)
	})
}

func TestNoLoginViewIssue(t *testing.T) {
	prepareTestEnv(t)

	req := NewRequest(t, "GET", "/user2/repo1/issues/1")
	MakeRequest(t, req, http.StatusOK)
}

func testNewIssue(t *testing.T, session *TestSession, user, repo, title, content string) string {

	req := NewRequest(t, "GET", path.Join(user, repo, "issues", "new"))
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find("form.ui.form").Attr("action")
	assert.True(t, exists, "The template has changed")
	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"_csrf":   htmlDoc.GetCSRF(),
		"title":   title,
		"content": content,
	})
	resp = session.MakeRequest(t, req, http.StatusFound)

	issueURL := test.RedirectURL(resp)
	req = NewRequest(t, "GET", issueURL)
	resp = session.MakeRequest(t, req, http.StatusOK)

	htmlDoc = NewHTMLParser(t, resp.Body)
	val := htmlDoc.doc.Find("#issue-title").Text()
	assert.Equal(t, title, val)
	val = htmlDoc.doc.Find(".comment-list .comments .comment .render-content p").First().Text()
	assert.Equal(t, content, val)

	return issueURL
}

func testIssueAddComment(t *testing.T, session *TestSession, issueURL, content, status string) {

	req := NewRequest(t, "GET", issueURL)
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find("#comment-form").Attr("action")
	assert.True(t, exists, "The template has changed")

	commentCount := htmlDoc.doc.Find(".comment-list .comments .comment .render-content").Length()

	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"_csrf":   htmlDoc.GetCSRF(),
		"content": content,
		"status":  status,
	})
	resp = session.MakeRequest(t, req, http.StatusFound)

	req = NewRequest(t, "GET", test.RedirectURL(resp))
	resp = session.MakeRequest(t, req, http.StatusOK)

	htmlDoc = NewHTMLParser(t, resp.Body)

	val := htmlDoc.doc.Find(".comment-list .comments .comment .render-content p").Eq(commentCount).Text()
	assert.Equal(t, content, val)
}

func TestNewIssue(t *testing.T) {
	prepareTestEnv(t)
	session := loginUser(t, "user2")
	testNewIssue(t, session, "user2", "repo1", "Title", "Description")
}

func TestIssueCommentClose(t *testing.T) {
	prepareTestEnv(t)
	session := loginUser(t, "user2")
	issueURL := testNewIssue(t, session, "user2", "repo1", "Title", "Description")
	testIssueAddComment(t, session, issueURL, "Test comment 1", "")
	testIssueAddComment(t, session, issueURL, "Test comment 2", "")
	testIssueAddComment(t, session, issueURL, "Test comment 3", "close")

	// Validate that issue content has not been updated
	req := NewRequest(t, "GET", issueURL)
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	val := htmlDoc.doc.Find(".comment-list .comments .comment .render-content p").First().Text()
	assert.Equal(t, "Description", val)
}
