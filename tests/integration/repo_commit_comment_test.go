// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestCommitInlineComment(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	const rootSHA = "65f1bf27bc3bf70f64657658635e66094edbcb4d" // adds README.md with 3 lines
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

	t.Run("CreateListDelete", func(t *testing.T) {
		body := postComment(rootSHA, "proposed", "1", "first commit comment")
		assert.Contains(t, body, "first commit comment")

		reply := postComment(rootSHA, "proposed", "1", "second commit comment")
		assert.Contains(t, reply, "first commit comment")
		assert.Contains(t, reply, "second commit comment")

		resp := session.MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/commit/"+rootSHA), http.StatusOK)
		page := NewHTMLParser(t, resp.Body)
		assert.Equal(t, 2, page.Find(".comment-code-cloud .comment").Length())
		assert.NotZero(t, page.Find(".add-code-comment").Length())

		splitResp := session.MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/commit/"+rootSHA+"?style=split"), http.StatusOK)
		split := NewHTMLParser(t, splitResp.Body)
		assert.NotZero(t, split.Find(".add-code-comment").Length(), "split diff should expose inline comment controls")
		assert.Contains(t, strings.TrimSpace(split.Find(".commit-conversation .comment-body").Text()), "first commit comment")

		comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{Content: "second commit comment", Type: issues_model.CommentTypeCommitComment})
		url := fmt.Sprintf("/user2/repo1/commit/%s/comment/%d/delete", rootSHA, comment.ID)
		session.MakeRequest(t, NewRequest(t, "POST", url), http.StatusOK)
		unittest.AssertNotExistsBean(t, &issues_model.Comment{ID: comment.ID})
		unittest.AssertNotExistsBean(t, &issues_model.CommitComment{CommentID: comment.ID})
		session.MakeRequest(t, NewRequest(t, "POST", url), http.StatusNotFound)
	})
}
