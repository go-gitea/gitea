// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/indexer/issues"
	"code.gitea.io/gitea/modules/references"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
)

func getIssuesSelection(t testing.TB, htmlDoc *HTMLDoc) *goquery.Selection {
	issueList := htmlDoc.doc.Find("#issue-list")
	assert.EqualValues(t, 1, issueList.Length())
	return issueList.Find(".flex-item").Find(".issue-title")
}

func getIssue(t *testing.T, repoID int64, issueSelection *goquery.Selection) *issues_model.Issue {
	href, exists := issueSelection.Attr("href")
	assert.True(t, exists)
	indexStr := href[strings.LastIndexByte(href, '/')+1:]
	index, err := strconv.Atoi(indexStr)
	assert.NoError(t, err, "Invalid issue href: %s", href)
	return unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repoID, Index: int64(index)})
}

func assertMatch(t testing.TB, issue *issues_model.Issue, keyword string) {
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
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/user2/repo1/issues")
	MakeRequest(t, req, http.StatusOK)
}

func TestViewIssuesSortByType(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	session := loginUser(t, user.Name)
	req := NewRequest(t, "GET", repo.Link()+"/issues?type=created_by")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	issuesSelection := getIssuesSelection(t, htmlDoc)
	expectedNumIssues := unittest.GetCount(t,
		&issues_model.Issue{RepoID: repo.ID, PosterID: user.ID},
		unittest.Cond("is_closed=?", false),
		unittest.Cond("is_pull=?", false),
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
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{
		RepoID: repo.ID,
		Index:  1,
	})
	issues.UpdateIssueIndexer(t.Context(), issue.ID)
	time.Sleep(time.Second * 1)
	const keyword = "first"
	req := NewRequestf(t, "GET", "%s/issues?q=%s", repo.Link(), keyword)
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
	defer tests.PrepareTestEnv(t)()

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
	resp = session.MakeRequest(t, req, http.StatusOK)

	issueURL := test.RedirectURL(resp)
	req = NewRequest(t, "GET", issueURL)
	resp = session.MakeRequest(t, req, http.StatusOK)

	htmlDoc = NewHTMLParser(t, resp.Body)
	val := htmlDoc.doc.Find("#issue-title-display").Text()
	assert.Contains(t, val, title)
	val = htmlDoc.doc.Find(".comment .render-content p").First().Text()
	assert.Equal(t, content, val)

	return issueURL
}

func testIssueAddComment(t *testing.T, session *TestSession, issueURL, content, status string) int64 {
	req := NewRequest(t, "GET", issueURL)
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find("#comment-form").Attr("action")
	assert.True(t, exists, "The template has changed")

	commentCount := htmlDoc.doc.Find(".comment-list .comment .render-content").Length()

	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"_csrf":   htmlDoc.GetCSRF(),
		"content": content,
		"status":  status,
	})
	resp = session.MakeRequest(t, req, http.StatusOK)

	req = NewRequest(t, "GET", test.RedirectURL(resp))
	resp = session.MakeRequest(t, req, http.StatusOK)

	htmlDoc = NewHTMLParser(t, resp.Body)

	val := strings.TrimSpace(htmlDoc.doc.Find(".comment-list .comment .render-content").Eq(commentCount).Text())
	assert.Equal(t, content, val)

	idAttr, has := htmlDoc.doc.Find(".comment-list .comment").Eq(commentCount).Attr("id")
	idStr := idAttr[strings.LastIndexByte(idAttr, '-')+1:]
	assert.True(t, has)
	id, err := strconv.Atoi(idStr)
	assert.NoError(t, err)
	return int64(id)
}

func TestNewIssue(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")
	testNewIssue(t, session, "user2", "repo1", "Title", "Description")
}

func TestEditIssue(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")
	issueURL := testNewIssue(t, session, "user2", "repo1", "Title", "Description")

	req := NewRequestWithValues(t, "POST", fmt.Sprintf("%s/content", issueURL), map[string]string{
		"_csrf":   GetUserCSRFToken(t, session),
		"content": "modified content",
		"context": fmt.Sprintf("/%s/%s", "user2", "repo1"),
	})
	session.MakeRequest(t, req, http.StatusOK)

	req = NewRequestWithValues(t, "POST", fmt.Sprintf("%s/content", issueURL), map[string]string{
		"_csrf":   GetUserCSRFToken(t, session),
		"content": "modified content",
		"context": fmt.Sprintf("/%s/%s", "user2", "repo1"),
	})
	session.MakeRequest(t, req, http.StatusBadRequest)

	req = NewRequestWithValues(t, "POST", fmt.Sprintf("%s/content", issueURL), map[string]string{
		"_csrf":           GetUserCSRFToken(t, session),
		"content":         "modified content",
		"content_version": "1",
		"context":         fmt.Sprintf("/%s/%s", "user2", "repo1"),
	})
	session.MakeRequest(t, req, http.StatusOK)
}

func TestIssueCommentClose(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")
	issueURL := testNewIssue(t, session, "user2", "repo1", "Title", "Description")
	testIssueAddComment(t, session, issueURL, "Test comment 1", "")
	testIssueAddComment(t, session, issueURL, "Test comment 2", "")
	testIssueAddComment(t, session, issueURL, "Test comment 3", "close")

	// Validate that issue content has not been updated
	req := NewRequest(t, "GET", issueURL)
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	val := htmlDoc.doc.Find(".comment-list .comment .render-content p").First().Text()
	assert.Equal(t, "Description", val)
}

func TestIssueCommentDelete(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")
	issueURL := testNewIssue(t, session, "user2", "repo1", "Title", "Description")
	comment1 := "Test comment 1"
	commentID := testIssueAddComment(t, session, issueURL, comment1, "")
	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: commentID})
	assert.Equal(t, comment1, comment.Content)

	// Using the ID of a comment that does not belong to the repository must fail
	req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/comments/%d/delete", "user5", "repo4", commentID), map[string]string{
		"_csrf": GetUserCSRFToken(t, session),
	})
	session.MakeRequest(t, req, http.StatusNotFound)
	req = NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/comments/%d/delete", "user2", "repo1", commentID), map[string]string{
		"_csrf": GetUserCSRFToken(t, session),
	})
	session.MakeRequest(t, req, http.StatusOK)
	unittest.AssertNotExistsBean(t, &issues_model.Comment{ID: commentID})
}

func TestIssueCommentUpdate(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")
	issueURL := testNewIssue(t, session, "user2", "repo1", "Title", "Description")
	comment1 := "Test comment 1"
	commentID := testIssueAddComment(t, session, issueURL, comment1, "")

	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: commentID})
	assert.Equal(t, comment1, comment.Content)

	modifiedContent := comment.Content + "MODIFIED"

	// Using the ID of a comment that does not belong to the repository must fail
	req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/comments/%d", "user5", "repo4", commentID), map[string]string{
		"_csrf":   GetUserCSRFToken(t, session),
		"content": modifiedContent,
	})
	session.MakeRequest(t, req, http.StatusNotFound)

	req = NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/comments/%d", "user2", "repo1", commentID), map[string]string{
		"_csrf":   GetUserCSRFToken(t, session),
		"content": modifiedContent,
	})
	session.MakeRequest(t, req, http.StatusOK)

	comment = unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: commentID})
	assert.Equal(t, modifiedContent, comment.Content)
}

func TestIssueCommentUpdateSimultaneously(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")
	issueURL := testNewIssue(t, session, "user2", "repo1", "Title", "Description")
	comment1 := "Test comment 1"
	commentID := testIssueAddComment(t, session, issueURL, comment1, "")

	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: commentID})
	assert.Equal(t, comment1, comment.Content)

	modifiedContent := comment.Content + "MODIFIED"

	req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/comments/%d", "user2", "repo1", commentID), map[string]string{
		"_csrf":   GetUserCSRFToken(t, session),
		"content": modifiedContent,
	})
	session.MakeRequest(t, req, http.StatusOK)

	modifiedContent = comment.Content + "2"

	req = NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/comments/%d", "user2", "repo1", commentID), map[string]string{
		"_csrf":   GetUserCSRFToken(t, session),
		"content": modifiedContent,
	})
	session.MakeRequest(t, req, http.StatusBadRequest)

	req = NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/comments/%d", "user2", "repo1", commentID), map[string]string{
		"_csrf":           GetUserCSRFToken(t, session),
		"content":         modifiedContent,
		"content_version": "1",
	})
	session.MakeRequest(t, req, http.StatusOK)

	comment = unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: commentID})
	assert.Equal(t, modifiedContent, comment.Content)
	assert.Equal(t, 2, comment.ContentVersion)
}

func TestIssueReaction(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")
	issueURL := testNewIssue(t, session, "user2", "repo1", "Title", "Description")

	req := NewRequest(t, "GET", issueURL)
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	req = NewRequestWithValues(t, "POST", path.Join(issueURL, "/reactions/react"), map[string]string{
		"_csrf":   htmlDoc.GetCSRF(),
		"content": "8ball",
	})
	session.MakeRequest(t, req, http.StatusInternalServerError)
	req = NewRequestWithValues(t, "POST", path.Join(issueURL, "/reactions/react"), map[string]string{
		"_csrf":   htmlDoc.GetCSRF(),
		"content": "eyes",
	})
	session.MakeRequest(t, req, http.StatusOK)
	req = NewRequestWithValues(t, "POST", path.Join(issueURL, "/reactions/unreact"), map[string]string{
		"_csrf":   htmlDoc.GetCSRF(),
		"content": "eyes",
	})
	session.MakeRequest(t, req, http.StatusOK)
}

func TestIssueCrossReference(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// Issue that will be referenced
	_, issueBase := testIssueWithBean(t, "user2", 1, "Title", "Description")

	// Ref from issue title
	issueRefURL, issueRef := testIssueWithBean(t, "user2", 1, fmt.Sprintf("Title ref #%d", issueBase.Index), "Description")
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{
		IssueID:      issueBase.ID,
		RefRepoID:    1,
		RefIssueID:   issueRef.ID,
		RefCommentID: 0,
		RefIsPull:    false,
		RefAction:    references.XRefActionNone,
	})

	// Edit title, neuter ref
	testIssueChangeInfo(t, "user2", issueRefURL, "title", "Title no ref")
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{
		IssueID:      issueBase.ID,
		RefRepoID:    1,
		RefIssueID:   issueRef.ID,
		RefCommentID: 0,
		RefIsPull:    false,
		RefAction:    references.XRefActionNeutered,
	})

	// Ref from issue content
	issueRefURL, issueRef = testIssueWithBean(t, "user2", 1, "TitleXRef", fmt.Sprintf("Description ref #%d", issueBase.Index))
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{
		IssueID:      issueBase.ID,
		RefRepoID:    1,
		RefIssueID:   issueRef.ID,
		RefCommentID: 0,
		RefIsPull:    false,
		RefAction:    references.XRefActionNone,
	})

	// Edit content, neuter ref
	testIssueChangeInfo(t, "user2", issueRefURL, "content", "Description no ref")
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{
		IssueID:      issueBase.ID,
		RefRepoID:    1,
		RefIssueID:   issueRef.ID,
		RefCommentID: 0,
		RefIsPull:    false,
		RefAction:    references.XRefActionNeutered,
	})

	// Ref from a comment
	session := loginUser(t, "user2")
	commentID := testIssueAddComment(t, session, issueRefURL, fmt.Sprintf("Adding ref from comment #%d", issueBase.Index), "")
	comment := &issues_model.Comment{
		IssueID:      issueBase.ID,
		RefRepoID:    1,
		RefIssueID:   issueRef.ID,
		RefCommentID: commentID,
		RefIsPull:    false,
		RefAction:    references.XRefActionNone,
	}
	unittest.AssertExistsAndLoadBean(t, comment)

	// Ref from a different repository
	_, issueRef = testIssueWithBean(t, "user12", 10, "TitleXRef", fmt.Sprintf("Description ref user2/repo1#%d", issueBase.Index))
	unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{
		IssueID:      issueBase.ID,
		RefRepoID:    10,
		RefIssueID:   issueRef.ID,
		RefCommentID: 0,
		RefIsPull:    false,
		RefAction:    references.XRefActionNone,
	})
}

func testIssueWithBean(t *testing.T, user string, repoID int64, title, content string) (string, *issues_model.Issue) {
	session := loginUser(t, user)
	issueURL := testNewIssue(t, session, user, fmt.Sprintf("repo%d", repoID), title, content)
	indexStr := issueURL[strings.LastIndexByte(issueURL, '/')+1:]
	index, err := strconv.Atoi(indexStr)
	assert.NoError(t, err, "Invalid issue href: %s", issueURL)
	issue := &issues_model.Issue{RepoID: repoID, Index: int64(index)}
	unittest.AssertExistsAndLoadBean(t, issue)
	return issueURL, issue
}

func testIssueChangeInfo(t *testing.T, user, issueURL, info, value string) {
	session := loginUser(t, user)

	req := NewRequest(t, "GET", issueURL)
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	req = NewRequestWithValues(t, "POST", path.Join(issueURL, info), map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
		info:    value,
	})
	_ = session.MakeRequest(t, req, http.StatusOK)
}

func TestIssueRedirect(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")

	// Test external tracker where style not set (shall default numeric)
	req := NewRequest(t, "GET", path.Join("org26", "repo_external_tracker", "issues", "1"))
	resp := session.MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, "https://tracker.com/org26/repo_external_tracker/issues/1", test.RedirectURL(resp))

	// Test external tracker with numeric style
	req = NewRequest(t, "GET", path.Join("org26", "repo_external_tracker_numeric", "issues", "1"))
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, "https://tracker.com/org26/repo_external_tracker_numeric/issues/1", test.RedirectURL(resp))

	// Test external tracker with alphanumeric style (for a pull request)
	req = NewRequest(t, "GET", path.Join("org26", "repo_external_tracker_alpha", "issues", "1"))
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, "/"+path.Join("org26", "repo_external_tracker_alpha", "pulls", "1"), test.RedirectURL(resp))
}

func TestSearchIssues(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	expectedIssueCount := 20 // from the fixtures
	if expectedIssueCount > setting.UI.IssuePagingNum {
		expectedIssueCount = setting.UI.IssuePagingNum
	}

	link, _ := url.Parse("/issues/search")
	req := NewRequest(t, "GET", link.String())
	resp := session.MakeRequest(t, req, http.StatusOK)
	var apiIssues []*api.Issue
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, expectedIssueCount)

	since := "2000-01-01T00:50:01+00:00" // 946687801
	before := time.Unix(999307200, 0).Format(time.RFC3339)
	query := url.Values{}
	query.Add("since", since)
	query.Add("before", before)
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, 11)
	query.Del("since")
	query.Del("before")

	query.Add("state", "closed")
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, 2)

	query.Set("state", "all")
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	assert.EqualValues(t, "22", resp.Header().Get("X-Total-Count"))
	assert.Len(t, apiIssues, 20)

	query.Add("limit", "5")
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	assert.EqualValues(t, "22", resp.Header().Get("X-Total-Count"))
	assert.Len(t, apiIssues, 5)

	query = url.Values{"assigned": {"true"}, "state": {"all"}}
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, 2)

	query = url.Values{"milestones": {"milestone1"}, "state": {"all"}}
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, 1)

	query = url.Values{"milestones": {"milestone1,milestone3"}, "state": {"all"}}
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, 2)

	query = url.Values{"owner": {"user2"}} // user
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, 8)

	query = url.Values{"owner": {"org3"}} // organization
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, 5)

	query = url.Values{"owner": {"org3"}, "team": {"team1"}} // organization + team
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, 2)
}

func TestSearchIssuesWithLabels(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	expectedIssueCount := 20 // from the fixtures
	if expectedIssueCount > setting.UI.IssuePagingNum {
		expectedIssueCount = setting.UI.IssuePagingNum
	}

	session := loginUser(t, "user1")
	link, _ := url.Parse("/issues/search")
	query := url.Values{}
	var apiIssues []*api.Issue

	link.RawQuery = query.Encode()
	req := NewRequest(t, "GET", link.String())
	resp := session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, expectedIssueCount)

	query.Add("labels", "label1")
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, 2)

	// multiple labels
	query.Set("labels", "label1,label2")
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, 2)

	// an org label
	query.Set("labels", "orglabel4")
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, 1)

	// org and repo label
	query.Set("labels", "label2,orglabel4")
	query.Add("state", "all")
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, 2)

	// org and repo label which share the same issue
	query.Set("labels", "label1,orglabel4")
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, 2)
}

func TestGetIssueInfo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 10})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	assert.NoError(t, issue.LoadAttributes(db.DefaultContext))
	assert.Equal(t, int64(1019307200), int64(issue.DeadlineUnix))
	assert.Equal(t, api.StateOpen, issue.State())

	session := loginUser(t, owner.Name)

	urlStr := fmt.Sprintf("/%s/%s/issues/%d/info", owner.Name, repo.Name, issue.Index)
	req := NewRequest(t, "GET", urlStr)
	resp := session.MakeRequest(t, req, http.StatusOK)
	var respStruct struct {
		ConvertedIssue api.Issue
		RenderedLabels template.HTML
	}
	DecodeJSON(t, resp, &respStruct)

	assert.EqualValues(t, issue.ID, respStruct.ConvertedIssue.ID)
	assert.Contains(t, string(respStruct.RenderedLabels), `"labels-list"`)
}

func TestUpdateIssueDeadline(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	issueBefore := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 10})
	repoBefore := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issueBefore.RepoID})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repoBefore.OwnerID})
	assert.NoError(t, issueBefore.LoadAttributes(db.DefaultContext))
	assert.Equal(t, "2002-04-20", issueBefore.DeadlineUnix.FormatDate())
	assert.Equal(t, api.StateOpen, issueBefore.State())

	session := loginUser(t, owner.Name)
	urlStr := fmt.Sprintf("%s/%s/issues/%d/deadline?_csrf=%s", owner.Name, repoBefore.Name, issueBefore.Index, GetUserCSRFToken(t, session))

	req := NewRequestWithValues(t, "POST", urlStr, map[string]string{"deadline": "2022-04-06"})
	session.MakeRequest(t, req, http.StatusOK)
	issueAfter := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 10})
	assert.EqualValues(t, "2022-04-06", issueAfter.DeadlineUnix.FormatDate())

	req = NewRequestWithValues(t, "POST", urlStr, map[string]string{"deadline": ""})
	session.MakeRequest(t, req, http.StatusOK)
	issueAfter = unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 10})
	assert.True(t, issueAfter.DeadlineUnix.IsZero())
}

func TestIssueReferenceURL(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})

	req := NewRequest(t, "GET", fmt.Sprintf("%s/issues/%d", repo.FullName(), issue.Index))
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	// the "reference" uses relative URLs, then JS code will convert them to absolute URLs for current origin, in case users are using multiple domains
	ref, _ := htmlDoc.Find(`.timeline-item.comment.first .reference-issue`).Attr("data-reference")
	assert.EqualValues(t, "/user2/repo1/issues/1#issue-1", ref)

	ref, _ = htmlDoc.Find(`.timeline-item.comment:not(.first) .reference-issue`).Attr("data-reference")
	assert.EqualValues(t, "/user2/repo1/issues/1#issuecomment-2", ref)
}
