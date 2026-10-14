// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	repo_service "gitea.dev/services/repository"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommitInlineComment(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	const rootSHA = "65f1bf27bc3bf70f64657658635e66094edbcb4d" // adds README.md with 3 lines
	const changeSHA = "5c050d3b6d2db231ab1f64e324f1b6b9a0b181c2"
	session := loginUser(t, "user2")

	postComment := func(sha, side, line, content string) string {
		req := NewRequestWithValues(t, "POST", "/user2/repo1/commit/"+sha+"/comment", map[string]string{
			"content": content,
			"path":    "README.md",
			"side":    side,
			"line":    line,
		})
		return session.MakeRequest(t, req, http.StatusOK).Body.String()
	}

	t.Run("ReplyKeepsWholeThread", func(t *testing.T) {
		assert.Contains(t, postComment(rootSHA, "proposed", "1", "first commit comment"), "first commit comment")

		// The response replaces the whole conversation holder client-side, so a
		// reply has to carry the entire thread and not just the new comment.
		reply := postComment(rootSHA, "proposed", "1", "second commit comment")
		assert.Contains(t, reply, "first commit comment")
		assert.Contains(t, reply, "second commit comment")
		assert.Contains(t, reply, "comment-form-reply", "the replacement holder has to keep its own reply form")
	})

	t.Run("CommentedLineHidesAddButton", func(t *testing.T) {
		resp := session.MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/commit/"+rootSHA), http.StatusOK)
		page := NewHTMLParser(t, resp.Body)
		assert.Equal(t, 2, page.Find(".comment-code-cloud .comment").Length())

		// Clicking "add comment" on a line that already has a conversation does
		// nothing, so the button must not be offered there.
		btn := page.Find(`.add-code-comment[data-side="right"][data-idx="1"]`)
		require.Equal(t, 1, btn.Length())
		assert.True(t, btn.HasClass("tw-invisible"))
	})

	t.Run("DeleteFollowsPosterOrAdmin", func(t *testing.T) {
		deleteButtons := func(user string) int {
			resp := loginUser(t, user).MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/commit/"+rootSHA), http.StatusOK)
			return NewHTMLParser(t, resp.Body).Find(".comment-code-cloud .delete-comment").Length()
		}
		assert.Equal(t, 2, deleteButtons("user2"), "the poster deletes their own comments")
		assert.Equal(t, 2, deleteButtons("user1"), "an admin deletes anyone's comments")
		assert.Equal(t, 0, deleteButtons("user5"), "other readers get no delete button")

		// The route has to agree with the buttons.
		comment := unittest.AssertExistsAndLoadBean(t, &repo_model.CommitComment{Content: "first commit comment"})
		url := fmt.Sprintf("/user2/repo1/commit/%s/comment/%d/delete", rootSHA, comment.ID)
		loginUser(t, "user5").MakeRequest(t, NewRequest(t, "POST", url), http.StatusForbidden)
		loginUser(t, "user1").MakeRequest(t, NewRequest(t, "POST", url), http.StatusOK)
		unittest.AssertNotExistsBean(t, &repo_model.CommitComment{ID: comment.ID})
	})

	t.Run("PreviousSideComment", func(t *testing.T) {
		// Old line 3 is the line the commit removed.
		assert.Contains(t, postComment(changeSHA, "previous", "3", "old side comment"), "old side comment")
		unittest.AssertExistsAndLoadBean(t, &repo_model.CommitComment{CommitSHA: changeSHA, Line: -3})
	})

	t.Run("LinePastEndOfFileIsRejected", func(t *testing.T) {
		// The parent README has 3 lines and the new one has 5, so previous line 4
		// only exists if the old side is wrongly read from the commit's own tree.
		for _, c := range []struct{ side, line string }{{"previous", "4"}, {"proposed", "6"}} {
			req := NewRequestWithValues(t, "POST", "/user2/repo1/commit/"+changeSHA+"/comment", map[string]string{
				"content": "past the end",
				"path":    "README.md",
				"side":    c.side,
				"line":    c.line,
			})
			session.MakeRequest(t, req, http.StatusBadRequest)
		}
		unittest.AssertNotExistsBean(t, &repo_model.CommitComment{Content: "past the end"})
		assert.Contains(t, postComment(changeSHA, "proposed", "5", "last line comment"), "last line comment")
	})

	t.Run("DeleteMissingCommentIsNotFound", func(t *testing.T) {
		comment := unittest.AssertExistsAndLoadBean(t, &repo_model.CommitComment{Content: "second commit comment"})
		url := fmt.Sprintf("/user2/repo1/commit/%s/comment/%d/delete", rootSHA, comment.ID)

		session.MakeRequest(t, NewRequest(t, "POST", url), http.StatusOK)
		unittest.AssertNotExistsBean(t, &repo_model.CommitComment{ID: comment.ID})
		session.MakeRequest(t, NewRequest(t, "POST", url), http.StatusNotFound)
	})

	t.Run("WikiCommitDiffHasNoCommentButtons", func(t *testing.T) {
		// The wiki commit view reuses the commit diff handler, but its comment
		// endpoint resolves SHAs against the code repo, so it can never work.
		const wikiSHA = "0dca5bd9b5d7ef937710e056f575e86c0184ba85"
		resp := session.MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/wiki/commit/"+wikiSHA), http.StatusOK)
		assert.Equal(t, 0, NewHTMLParser(t, resp.Body).Find(".add-code-comment").Length())
	})
}

// TestCommitInlineCommentOnExpandedContext covers the blob excerpt, which is
// rendered by its own handler: without the commit-comment data the expanded
// rows lose their buttons and any conversation inside the expanded region.
func TestCommitInlineCommentOnExpandedContext(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo, err := repo_service.CreateRepositoryDirectly(t.Context(), user2, user2, repo_service.CreateRepoOptions{
			Name:          "commit_comment_expand",
			Readme:        "Default",
			AutoInit:      true,
			DefaultBranch: "main",
		}, true)
		require.NoError(t, err)

		session := loginUser(t, user2.Name)
		testEditFile(t, session, user2.Name, repo.Name, "main", "README.md", strings.Repeat("a\n", 30))
		testEditFile(t, session, user2.Name, repo.Name, "main", "README.md", strings.Repeat("a\n", 29)+"b\n")

		resp := session.MakeRequest(t, NewRequest(t, "GET", "/"+repo.FullName()+"/commits/branch/main"), http.StatusOK)
		commitLink := NewHTMLParser(t, resp.Body).Find("#commits-table .commit-id-short").AttrOr("href", "")
		require.NotEmpty(t, commitLink)

		resp = session.MakeRequest(t, NewRequest(t, "GET", commitLink), http.StatusOK)
		expander := NewHTMLParser(t, resp.Body).Find(`button.code-expander-button[data-fetch-url]`)
		require.NotZero(t, expander.Length())
		excerptURL := expander.Eq(0).AttrOr("data-fetch-url", "")

		// The excerpt is a table fragment, so match on the raw markup rather
		// than parsing it out of table context.
		excerptBody := session.MakeRequest(t, NewRequest(t, "GET", excerptURL), http.StatusOK).Body.String()
		assert.Contains(t, excerptBody, "add-code-comment", "expanded rows must offer the comment button")

		lineNums := regexp.MustCompile(`data-line-num="(\d+)"`).FindAllStringSubmatch(excerptBody, -1)
		require.NotEmpty(t, lineNums)
		expandedLine := lineNums[len(lineNums)-1][1]

		req := NewRequestWithValues(t, "POST", commitLink+"/comment", map[string]string{
			"content": "comment on expanded line",
			"path":    "README.md",
			"side":    "proposed",
			"line":    expandedLine,
		})
		session.MakeRequest(t, req, http.StatusOK)

		body := session.MakeRequest(t, NewRequest(t, "GET", excerptURL), http.StatusOK).Body.String()
		assert.Contains(t, body, "comment on expanded line")

		// Expanded lines are unchanged, so like the PR excerpt they only take
		// comments on the new side, in both columns of the split view.
		require.Contains(t, excerptURL, "style=unified")
		splitBody := session.MakeRequest(t, NewRequest(t, "GET", strings.Replace(excerptURL, "style=unified", "style=split", 1)), http.StatusOK).Body.String()
		assert.Contains(t, splitBody, "add-code-comment-right")
		assert.NotContains(t, splitBody, "add-code-comment-left")
	})
}
