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
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/git"
	"gitea.dev/modules/queue"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommitInlineComment(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _u *url.URL) {
		const rootSHA = "65f1bf27bc3bf70f64657658635e66094edbcb4d"
		const changeSHA = "5c050d3b6d2db231ab1f64e324f1b6b9a0b181c2"
		const rootCommitLink = "/user2/repo1/commit/" + rootSHA
		session, admin := loginUser(t, "user4"), loginUser(t, "user2")
		getPage := func(session *TestSession, link string) *HTMLDoc {
			t.Helper()
			return NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", link), http.StatusOK).Body)
		}
		postComment := func(session *TestSession, sha, side, line, content string, status int) string {
			t.Helper()
			return session.MakeRequest(t, NewRequestWithValues(t, "POST", "/user2/repo1/commit/"+sha+"/comment", map[string]string{
				"content": content,
				"path":    "README.md",
				"side":    side,
				"line":    line,
			}), status).Body.String()
		}

		postComment(session, rootSHA, "proposed", "1", "**first commit comment** @user5", http.StatusOK)
		reply := postComment(session, rootSHA, "proposed", "1", "second commit comment", http.StatusOK)
		assert.Contains(t, reply, "<strong>first commit comment</strong>")
		assert.Contains(t, reply, "second commit comment")
		assert.Contains(t, reply, "comment-form-reply")

		page := getPage(session, rootCommitLink)
		comments := page.Find(".comment-code-cloud .comment")
		require.Equal(t, 2, comments.Length())
		assert.Contains(t, comments.Find(".render-content").Text(), "first commit comment")
		assert.Contains(t, comments.Find(".render-content").Text(), "second commit comment")
		assert.True(t, page.Find(`.add-code-comment[data-side="right"][data-idx="1"]`).HasClass("tw-invisible"))

		commentID := strings.TrimPrefix(comments.First().AttrOr("id", ""), "issuecomment-")
		require.NotEmpty(t, commentID)
		assert.Contains(t, comments.First().Find(".menu [data-clipboard-text]").AttrOr("data-clipboard-text", ""), rootCommitLink+"#issuecomment-"+commentID)
		assert.Zero(t, comments.Find(`[data-global-click="onConvContentEdit"]`).Length())

		other := loginUser(t, "user5")
		assert.Equal(t, 2, comments.Find(`[data-global-click="onConvContentDelete"]`).Length())
		for viewer, expected := range map[*TestSession]int{admin: 2, other: 0} {
			assert.Equal(t, expected, getPage(viewer, rootCommitLink).Find(`.comment-code-cloud [data-global-click="onConvContentDelete"]`).Length())
		}

		session.MakeRequest(t, NewRequest(t, "POST", "/user2/repo1/comments/"+commentID+"/delete"), http.StatusNotFound)
		deleteURL := rootCommitLink + "/comment/" + commentID + "/delete"
		other.MakeRequest(t, NewRequest(t, "POST", deleteURL), http.StatusForbidden)
		admin.MakeRequest(t, NewRequest(t, "POST", deleteURL), http.StatusOK)
		session.MakeRequest(t, NewRequest(t, "POST", deleteURL), http.StatusNotFound)

		comment := postComment(session, changeSHA, "previous", "3", "old side comment", http.StatusOK)
		assert.Equal(t, 1, NewHTMLParser(t, strings.NewReader(comment)).Find(`.conversation-holder[data-side="left"][data-idx="3"]`).Length())

		postComment(loginUser(t, "user29"), rootSHA, "proposed", "2", "blocked comment", http.StatusBadRequest)
		unittest.AssertNotExistsBean(t, &issues_model.Comment{Content: "blocked comment"})

		assert.Zero(t, getPage(session, "/user2/repo1/wiki/commit/0dca5bd9b5d7ef937710e056f575e86c0184ba85").Find(".add-code-comment").Length())

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		testEditFile(t, admin, "user2", repo.Name, repo.DefaultBranch, "README.md", strings.Repeat("a\n", 30))
		testEditFile(t, admin, "user2", repo.Name, repo.DefaultBranch, "README.md", strings.Repeat("a\n", 29)+"b\n")
		sha, err := git.GetBranchCommitID(t.Context(), repo, repo.DefaultBranch)
		require.NoError(t, err)
		commitLink := "/" + repo.FullName() + "/commit/" + sha

		excerptURL := getPage(admin, commitLink).Find("button.code-expander-button[data-fetch-url]").First().AttrOr("data-fetch-url", "")
		require.Contains(t, excerptURL, "commit_diff=1")

		line := getPage(admin, excerptURL).Find(`.add-code-comment[data-side="right"]`).Last().AttrOr("data-idx", "")
		require.NotEmpty(t, line)

		comment = postComment(session, sha, "proposed", line, "comment on expanded line", http.StatusOK)
		commentID = strings.TrimPrefix(NewHTMLParser(t, strings.NewReader(comment)).Find(".comment").AttrOr("id", ""), "issuecomment-")
		require.NotEmpty(t, commentID)

		require.NoError(t, queue.GetManager().FlushAll(t.Context(), 5*time.Second))
		unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{UserID: 5, CommitID: rootSHA, Source: activities_model.NotificationSourceCommit})
		authorNotification := unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{UserID: 2, CommitID: sha, Status: activities_model.NotificationStatusUnread})
		assert.Equal(t, commentID, strconv.FormatInt(authorNotification.CommentID, 10))

		assert.Contains(t, getPage(admin, commitLink).Find("button.code-expander-button").First().AttrOr("data-hidden-comment-ids", ""), ","+commentID+",")
		unittest.AssertExistsAndLoadBean(t, &activities_model.Notification{ID: authorNotification.ID, Status: activities_model.NotificationStatusRead})
		assert.Contains(t, getPage(admin, excerptURL).Find(".render-content").Text(), "comment on expanded line")

		buttons := getPage(admin, strings.Replace(excerptURL, "style=unified", "style=split", 1)).Find(".add-code-comment")
		require.Positive(t, buttons.Length())
		assert.Equal(t, buttons.Length(), buttons.Filter(`[data-side="right"]`).Length())
	})
}
