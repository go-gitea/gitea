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
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommitInlineComment(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	const rootSHA = "65f1bf27bc3bf70f64657658635e66094edbcb4d"
	const changeSHA = "5c050d3b6d2db231ab1f64e324f1b6b9a0b181c2"
	const rootCommitLink = "/user2/repo1/commit/" + rootSHA
	session, admin := loginUser(t, "user4"), loginUser(t, "user2")
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

	page := getPage(admin, rootCommitLink)
	comments := page.Find(".comment-code-cloud .comment")
	require.Equal(t, 2, comments.Length())
	assert.Contains(t, comments.Find(".render-content").Text(), "first commit comment")
	assert.Contains(t, comments.Find(".render-content").Text(), "second commit comment")
	assert.Equal(t, 2, comments.Find(`[data-global-click="onConvContentEdit"]`).Length())
	assert.Equal(t, 2, comments.Find(`[data-global-click="onConvContentDelete"]`).Length())
	assert.True(t, page.Find(`.add-code-comment[data-side="right"][data-idx="1"]`).HasClass("tw-invisible"))

	commentID := strings.TrimPrefix(comments.First().AttrOr("id", ""), "issuecomment-")
	require.NotEmpty(t, commentID)
	assert.Contains(t, comments.First().Find(".menu [data-clipboard-text]").AttrOr("data-clipboard-text", ""), rootCommitLink+"#issuecomment-"+commentID)
	id, err := strconv.ParseInt(commentID, 10, 64)
	require.NoError(t, err)
	assert.Contains(t, comments.First().Find(".dropzone-attachments a").First().AttrOr("href", ""), "/attachments/"+uuid)
	MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/attachments/"+uuid), http.StatusOK)
	MakeRequest(t, NewRequest(t, "GET", "/attachments/"+uuid).AddTokenAuth(getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadIssue)), http.StatusForbidden)

	other := loginUser(t, "user5")
	assert.Zero(t, getPage(other, rootCommitLink).Find(`.comment-code-cloud [data-global-click="onConvContentEdit"]`).Length())
	commentLink := "/user2/repo1/comments/" + commentID
	editValues := url.Values{"content": {"edited commit comment"}, "content_version": {"0"}, "files[]": {uuid}}
	other.MakeRequest(t, NewRequestWithURLValues(t, "POST", commentLink, editValues), http.StatusForbidden)
	assert.Contains(t, session.MakeRequest(t, NewRequestWithURLValues(t, "POST", commentLink, editValues), http.StatusOK).Body.String(), "edited commit comment")
	assert.Zero(t, unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{UUID: uuid, CommentID: id}).IssueID)

	other.MakeRequest(t, NewRequestWithValues(t, "POST", commentLink+"/reactions/react", map[string]string{"content": "+1"}), http.StatusOK)
	unittest.AssertExistsAndLoadBean(t, &issues_model.Reaction{CommentID: id, UserID: 5, Type: "+1"})

	other.MakeRequest(t, NewRequest(t, "POST", commentLink+"/delete"), http.StatusForbidden)
	admin.MakeRequest(t, NewRequest(t, "POST", commentLink+"/delete"), http.StatusOK)
	session.MakeRequest(t, NewRequest(t, "POST", commentLink+"/delete"), http.StatusNotFound)

	comment := postComment(session, changeSHA, "previous", "3", "old side comment", http.StatusOK)
	assert.Equal(t, 1, NewHTMLParser(t, strings.NewReader(comment)).Find(`.conversation-holder[data-side="left"][data-idx="3"]`).Length())

	postComment(loginUser(t, "user29"), rootSHA, "proposed", "2", "blocked by owner", http.StatusBadRequest)
	unittest.AssertNotExistsBean(t, &issues_model.Comment{Content: "blocked by owner"})

	assert.Zero(t, getPage(session, "/user2/repo1/wiki/commit/0dca5bd9b5d7ef937710e056f575e86c0184ba85").Find(".add-code-comment").Length())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	require.NoError(t, git.ForceFastImport(t.Context(), repo, []git.FastImportCommit{
		{Ref: "refs/heads/commit-comments", Files: []git.FastImportFile{{Path: "README.md", Content: strings.Repeat("a\n", 30)}}},
		{Ref: "refs/heads/commit-comments", Parent: ":1", Author: &git.Signature{Name: "user4", Email: "user4@example.com"}, Files: []git.FastImportFile{{Path: "README.md", Content: strings.Repeat("a\n", 29) + "b\n"}}},
	}))
	sha, err := git.GetBranchCommitID(t.Context(), repo, "commit-comments")
	require.NoError(t, err)
	commitLink := "/" + repo.FullName() + "/commit/" + sha

	excerptURL := getPage(admin, commitLink).Find("button.code-expander-button[data-fetch-url]").First().AttrOr("data-fetch-url", "")
	require.Contains(t, excerptURL, "commit_diff=1")
	getPage(admin, strings.Replace(excerptURL, "commit_diff=1", "pull_issue_index=2", 1))

	line := getPage(admin, excerptURL).Find(`.add-code-comment[data-side="right"]`).Last().AttrOr("data-idx", "")
	require.NotEmpty(t, line)

	comment = postComment(other, sha, "proposed", line, "comment on expanded line", http.StatusOK)
	commentID = strings.TrimPrefix(NewHTMLParser(t, strings.NewReader(comment)).Find(".comment").AttrOr("id", ""), "issuecomment-")
	require.NotEmpty(t, commentID)

	require.NoError(t, queue.GetManager().FlushAll(t.Context(), 5*time.Second))
	unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{UserID: 5, CommitID: rootSHA, Source: activities_model.NotificationSourceCommit})
	authorNotification := unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{UserID: 4, CommitID: sha, Status: activities_model.NotificationStatusUnread})
	assert.Equal(t, commentID, strconv.FormatInt(authorNotification.CommentID, 10))

	assert.Contains(t, getPage(session, commitLink).Find("button.code-expander-button").First().AttrOr("data-hidden-comment-ids", ""), ","+commentID+",")
	unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{ID: authorNotification.ID, Status: activities_model.NotificationStatusRead})
	assert.Contains(t, getPage(admin, excerptURL).Find(".render-content").Text(), "comment on expanded line")

	buttons := getPage(admin, strings.Replace(excerptURL, "style=unified", "style=split", 1)).Find(".add-code-comment")
	require.Positive(t, buttons.Length())
	assert.Equal(t, buttons.Length(), buttons.Filter(`[data-side="right"]`).Length())

	require.NoError(t, db.Insert(t.Context(), &user_model.Blocking{BlockerID: 4, BlockeeID: 5}))
	postComment(other, sha, "proposed", line, "blocked by author", http.StatusBadRequest)
	unittest.AssertNotExistsBean(t, &issues_model.Comment{Content: "blocked by author"})
}
