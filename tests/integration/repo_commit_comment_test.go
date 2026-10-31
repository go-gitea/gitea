// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	activities_model "gitea.dev/models/activities"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/queue"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommitInlineComment(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	const rootSHA = "65f1bf27bc3bf70f64657658635e66094edbcb4d"
	const changeSHA = "5c050d3b6d2db231ab1f64e324f1b6b9a0b181c2"
	const rootCommitLink = "/user2/repo1/commit/" + rootSHA
	session, owner := loginUser(t, "user4"), loginUser(t, "user2")
	getPage := func(session *TestSession, link string) *HTMLDoc {
		t.Helper()
		return NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", link), http.StatusOK).Body)
	}
	postComment := func(session *TestSession, sha, side, line, content string, status int, files ...string) string {
		t.Helper()
		return session.MakeRequest(t, NewRequestWithURLValues(t, "POST", "/user2/repo1/commit/"+sha+"/comment", url.Values{
			"content": {content},
			"path":    {"README.md"},
			"side":    {side},
			"line":    {line},
			"files":   files,
		}), status).Body.String()
	}

	uuid := testCreateIssueAttachment(t, session, "/user2/repo1", "image.png", testGeneratePngBytes(), http.StatusOK)
	postComment(session, rootSHA, "proposed", "1", "first commit comment @user5", http.StatusOK, uuid)
	assert.Contains(t, postComment(session, rootSHA, "proposed", "1", "second commit comment", http.StatusOK), "comment-form-reply")
	session.MakeRequest(t, NewRequestWithValues(t, "POST", rootCommitLink+"/comment", map[string]string{"content": "whole commit comment"}), http.StatusOK)

	page := getPage(owner, rootCommitLink)
	assert.Contains(t, page.Find(`[id^="commitcomment-"] .render-content`).Text(), "whole commit comment")
	comments := page.Find(".conversation-holder[data-path] .comment")
	require.Equal(t, 2, comments.Length())
	assert.Equal(t, 2, comments.Find(`[data-global-click="onConvContentEdit"]`).Length())
	assert.Equal(t, 2, comments.Find(`[data-global-click="onConvContentDelete"]`).Length())
	assert.True(t, page.Find(`.add-code-comment[data-side="right"][data-idx="1"]`).HasClass("tw-invisible"))

	commentID := strings.TrimPrefix(comments.First().AttrOr("id", ""), "r")
	require.NotEmpty(t, commentID)
	assert.Contains(t, comments.First().Find(".menu [data-clipboard-text]").AttrOr("data-clipboard-text", ""), rootCommitLink+"#r"+commentID)
	id, err := strconv.ParseInt(commentID, 10, 64)
	require.NoError(t, err)
	assert.Contains(t, comments.First().Find(".dropzone-attachments a").First().AttrOr("href", ""), "/attachments/"+uuid)
	MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/attachments/"+uuid), http.StatusOK)
	MakeRequest(t, NewRequest(t, "GET", "/attachments/"+uuid).AddTokenAuth(getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadIssue)), http.StatusForbidden)
	var apiComments []*api.CommitComment
	DecodeJSON(t, MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/commits/"+rootSHA+"/comments"), http.StatusOK), &apiComments)
	assert.Equal(t, uuid, apiComments[0].Attachments[0].UUID)

	other := loginUser(t, "user5")
	assert.Zero(t, getPage(other, rootCommitLink).Find(`.comment-code-cloud [data-global-click="onConvContentEdit"]`).Length())
	commentLink := "/user2/repo1/comments/" + commentID
	editValues := url.Values{"content": {"edited commit comment"}, "content_version": {"0"}, "files[]": {uuid}}
	other.MakeRequest(t, NewRequestWithURLValues(t, "POST", commentLink, editValues), http.StatusForbidden)
	assert.Contains(t, session.MakeRequest(t, NewRequestWithURLValues(t, "POST", commentLink, editValues), http.StatusOK).Body.String(), "edited commit comment")
	assert.Zero(t, unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{UUID: uuid, CommentID: id}).IssueID)

	historyLink := rootCommitLink + "/content-history"
	var overview struct {
		EditedHistoryCountMap map[string]int
	}
	DecodeJSON(t, session.MakeRequest(t, NewRequest(t, "GET", historyLink+"/overview"), http.StatusOK), &overview)
	assert.Equal(t, map[string]int{commentID: 2}, overview.EditedHistoryCountMap)
	var historyList struct {
		Results []any
	}
	DecodeJSON(t, session.MakeRequest(t, NewRequest(t, "GET", historyLink+"/list?comment_id="+commentID), http.StatusOK), &historyList)
	assert.Len(t, historyList.Results, 2)
	history := unittest.AssertExistsAndLoadBean(t, &issues_model.ContentHistory{CommentID: id, ContentText: "edited commit comment"})
	historyQuery := "?comment_id=" + commentID + "&history_id=" + strconv.FormatInt(history.ID, 10)
	var historyDetail struct {
		DiffHTML      string
		CanSoftDelete bool
	}
	DecodeJSON(t, session.MakeRequest(t, NewRequest(t, "GET", historyLink+"/detail"+historyQuery), http.StatusOK), &historyDetail)
	assert.Contains(t, historyDetail.DiffHTML, "edited")
	assert.True(t, historyDetail.CanSoftDelete)
	for _, link := range []string{"/user2/repo1/commit/" + changeSHA, "/user12/repo10/commit/" + rootSHA} {
		overview.EditedHistoryCountMap = nil
		DecodeJSON(t, owner.MakeRequest(t, NewRequest(t, "GET", link+"/content-history/overview"), http.StatusOK), &overview)
		assert.Empty(t, overview.EditedHistoryCountMap)
		owner.MakeRequest(t, NewRequest(t, "GET", link+"/content-history/list"+historyQuery), http.StatusNotFound)
		owner.MakeRequest(t, NewRequest(t, "GET", link+"/content-history/detail"+historyQuery), http.StatusNotFound)
		owner.MakeRequest(t, NewRequest(t, "POST", link+"/content-history/soft-delete"+historyQuery), http.StatusNotFound)
	}
	other.MakeRequest(t, NewRequest(t, "POST", historyLink+"/soft-delete"+historyQuery), http.StatusForbidden)
	session.MakeRequest(t, NewRequest(t, "POST", historyLink+"/soft-delete"+historyQuery), http.StatusOK)
	assert.True(t, unittest.AssertExistsAndLoadBean(t, &issues_model.ContentHistory{ID: history.ID}).IsDeleted)

	other.MakeRequest(t, NewRequestWithValues(t, "POST", commentLink+"/reactions/react", map[string]string{"content": "+1"}), http.StatusOK)

	owner.MakeRequest(t, NewRequest(t, "POST", commentLink+"/delete"), http.StatusOK)
	session.MakeRequest(t, NewRequest(t, "POST", commentLink+"/delete"), http.StatusNotFound)

	comment := postComment(session, changeSHA, "previous", "3", "old side comment", http.StatusOK)
	assert.Equal(t, 1, NewHTMLParser(t, strings.NewReader(comment)).Find(`.conversation-holder[data-side="left"][data-idx="3"]`).Length())

	assert.Equal(t, "2", strings.TrimSpace(getPage(other, "/user2/repo1/commits/branch/master").Find(`a[href$="/commit/`+rootSHA+`"] .octicon-comment`).Parent().Text()))

	assert.Zero(t, getPage(session, "/user2/repo1/wiki/commit/0dca5bd9b5d7ef937710e056f575e86c0184ba85").Find(".add-code-comment").Length())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	require.NoError(t, git.ForceFastImport(t.Context(), repo, []git.FastImportCommit{
		{Ref: "refs/heads/commit-comments", Files: []git.FastImportFile{{Path: "README.md", Content: strings.Repeat("a\n", 30)}}},
		{Ref: "refs/heads/commit-comments", Parent: ":1", Author: &git.Signature{Name: "user4", Email: "user4@example.com"}, Files: []git.FastImportFile{{Path: "README.md", Content: strings.Repeat("a\n", 29) + "b\n"}}},
	}))
	sha, err := git.GetBranchCommitID(t.Context(), repo, "commit-comments")
	require.NoError(t, err)
	commitLink := "/" + repo.FullName() + "/commit/" + sha

	excerptURL := getPage(owner, commitLink).Find("button.code-expander-button[data-fetch-url]").First().AttrOr("data-fetch-url", "")
	require.Contains(t, excerptURL, "commit_diff=1")
	getPage(owner, strings.Replace(excerptURL, "commit_diff=1", "pull_issue_index=2", 1))

	line := getPage(owner, excerptURL).Find(`.add-code-comment[data-side="right"]`).Last().AttrOr("data-idx", "")
	require.NotEmpty(t, line)

	comment = postComment(other, sha, "proposed", line, "comment on expanded line", http.StatusOK)
	commentID = strings.TrimPrefix(NewHTMLParser(t, strings.NewReader(comment)).Find(".comment").AttrOr("id", ""), "r")
	require.NotEmpty(t, commentID)

	require.NoError(t, queue.GetManager().FlushAll(t.Context(), 5*time.Second))
	unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{UserID: 5, CommitID: rootSHA, Source: activities_model.NotificationSourceCommit})
	authorNotification := unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{UserID: 4, CommitID: sha, Status: activities_model.NotificationStatusUnread})
	assert.Equal(t, commentID, strconv.FormatInt(authorNotification.CommentID, 10))

	assert.Contains(t, getPage(session, commitLink).Find("button.code-expander-button").First().AttrOr("data-hidden-comment-ids", ""), ","+commentID+",")
	unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{ID: authorNotification.ID, Status: activities_model.NotificationStatusRead})
	excerpt := getPage(owner, strings.Replace(excerptURL, "style=unified", "style=split", 1))
	assert.Contains(t, excerpt.Find(".render-content").Text(), "comment on expanded line")
	buttons := excerpt.Find(".add-code-comment")
	require.Positive(t, buttons.Length())
	assert.Equal(t, buttons.Length(), buttons.Filter(`[data-side="right"]`).Length())

	require.NoError(t, db.Insert(t.Context(), &user_model.Blocking{BlockerID: 4, BlockeeID: 5}))
	postComment(other, sha, "proposed", line, "blocked by author", http.StatusBadRequest)
	unittest.AssertNotExistsBean(t, &issues_model.Comment{Content: "blocked by author"})

	defer test.MockVariableValue(&setting.Repository.DisableCommitComments, true)()
	postComment(session, rootSHA, "proposed", "1", "disabled", http.StatusNotFound)
	disabledPage := getPage(session, rootCommitLink)
	assert.Zero(t, disabledPage.Find(".add-code-comment, .comment-code-cloud form").Length())
	assert.Positive(t, disabledPage.Find(".comment-code-cloud .comment").Length())
}
