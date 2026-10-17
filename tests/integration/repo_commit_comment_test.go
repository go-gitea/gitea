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
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	repo_service "gitea.dev/services/repository"
	files_service "gitea.dev/services/repository/files"
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

	t.Run("SharedContextMenu", func(t *testing.T) {
		resp := session.MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/commit/"+rootSHA), http.StatusOK)
		comment := NewHTMLParser(t, resp.Body).Find(".comment-code-cloud .comment").First()
		link, _ := comment.Find(".menu [data-clipboard-text]").Attr("data-clipboard-text")
		assert.Contains(t, link, "/user2/repo1/commit/"+rootSHA+"#commitcomment-")
		id, _ := comment.Attr("id")
		assert.Equal(t, 1, comment.Find("#"+id+"-raw.raw-content").Length(), "quote reply and copy source read the raw content")
		assert.Equal(t, 0, comment.Find(`[data-global-click="onConvContentEdit"]`).Length(), "commit comments have no edit endpoint")
	})

	t.Run("DeleteFollowsPosterOrAdmin", func(t *testing.T) {
		deleteButtons := func(user string) int {
			resp := loginUser(t, user).MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/commit/"+rootSHA), http.StatusOK)
			return NewHTMLParser(t, resp.Body).Find(`.comment-code-cloud [data-global-click="onConvContentDelete"]`).Length()
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

	t.Run("BlockedByOwnerIsRejected", func(t *testing.T) {
		// user2 owns repo1 and blocks user29 in the fixtures; repo1 is public,
		// so user29 can still read the code
		req := NewRequestWithValues(t, "POST", "/user2/repo1/commit/"+rootSHA+"/comment", map[string]string{
			"content": "blocked comment",
			"path":    "README.md",
			"side":    "proposed",
			"line":    "2",
		})
		loginUser(t, "user29").MakeRequest(t, req, http.StatusBadRequest)
		unittest.AssertNotExistsBean(t, &repo_model.CommitComment{Content: "blocked comment"})
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
		assert.Contains(t, excerptBody, "commit_diff=1", "the next expander must keep loading commit comments")

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
		sides := regexp.MustCompile(`add-code-comment[^"]*" data-side="(\w+)"`).FindAllStringSubmatch(splitBody, -1)
		require.NotEmpty(t, sides)
		for _, side := range sides {
			assert.Equal(t, "right", side[1])
		}
	})
}

// TestCommitInlineCommentDiffFiles checks the posted path against what the
// commit's diff actually shows, not just against the tree.
func TestCommitInlineCommentDiffFiles(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo, err := repo_service.CreateRepositoryDirectly(t.Context(), user2, user2, repo_service.CreateRepoOptions{
			Name:          "commit_comment_files",
			Readme:        "Default",
			AutoInit:      true,
			DefaultBranch: "main",
		}, true)
		require.NoError(t, err)

		change := func(lastCommitID string, files ...*files_service.ChangeRepoFile) string {
			resp, err := files_service.ChangeRepoFiles(t.Context(), repo, user2, &files_service.ChangeRepoFilesOptions{
				OldBranch:    "main",
				LastCommitID: lastCommitID,
				Message:      "change files",
				Files:        files,
			})
			require.NoError(t, err)
			return resp.Commit.SHA
		}
		lines := strings.Repeat("line\n", 9)
		first := change("",
			&files_service.ChangeRepoFile{Operation: "create", TreePath: "old.txt", ContentReader: strings.NewReader(lines + "last\n")},
			&files_service.ChangeRepoFile{Operation: "create", TreePath: "data.bin", ContentReader: strings.NewReader("a\x00b\n")},
			&files_service.ChangeRepoFile{Operation: "create", TreePath: "same.txt", ContentReader: strings.NewReader(lines)},
			&files_service.ChangeRepoFile{Operation: "create", TreePath: `a"b.txt`, ContentReader: strings.NewReader("x\n")},
			&files_service.ChangeRepoFile{Operation: "create", TreePath: ":notes.txt", ContentReader: strings.NewReader("x\n")},
		)
		sha := change(first,
			&files_service.ChangeRepoFile{Operation: "update", FromTreePath: "old.txt", TreePath: "new.txt", ContentReader: strings.NewReader(lines + "changed\n")},
			&files_service.ChangeRepoFile{Operation: "update", TreePath: "data.bin", ContentReader: strings.NewReader("a\x00c\n")},
			&files_service.ChangeRepoFile{Operation: "rename", FromTreePath: "same.txt", TreePath: "moved.txt"},
			&files_service.ChangeRepoFile{Operation: "update", TreePath: `a"b.txt`, ContentReader: strings.NewReader("y\n")},
			&files_service.ChangeRepoFile{Operation: "update", TreePath: ":notes.txt", ContentReader: strings.NewReader("y\n")},
		)

		session := loginUser(t, user2.Name)
		post := func(path, side, line string, status int) {
			req := NewRequestWithValues(t, "POST", "/"+repo.FullName()+"/commit/"+sha+"/comment", map[string]string{
				"content": "comment on " + path,
				"path":    path,
				"side":    side,
				"line":    line,
			})
			session.MakeRequest(t, req, status)
		}
		// The diff posts a renamed file's new name for both sides, but the
		// removed line only exists under the old name in the parent.
		post("new.txt", "previous", "10", http.StatusOK)
		// Neither of these has a row on the commit page to show the comment on.
		post("README.md", "proposed", "1", http.StatusBadRequest)
		post("data.bin", "proposed", "1", http.StatusBadRequest)
		post("moved.txt", "previous", "1", http.StatusBadRequest)
		// git quotes the first name in its plain output and reads the second as
		// pathspec magic, so neither may go through that route.
		post(`a"b.txt`, "proposed", "1", http.StatusOK)
		post(":notes.txt", "proposed", "1", http.StatusOK)
		// With MAX_GIT_DIFF_LINES at 0 the page still loads a file in full on
		// request, so the check must not depend on that limit.
		func() {
			defer test.MockVariableValue(&setting.Git.MaxGitDiffLines, 0)()
			post("new.txt", "proposed", "10", http.StatusOK)
		}()

		// ChangeRepoFiles cannot write symlinks, so build the type change by hand.
		// git splits it into a binary deletion and a text addition of one path.
		tmp, err := files_service.NewTemporaryUploadRepository(repo)
		require.NoError(t, err)
		defer tmp.Close()
		require.NoError(t, tmp.Clone(t.Context(), "main", true))
		require.NoError(t, tmp.SetDefaultIndex(t.Context()))
		target, err := tmp.HashObjectAndWrite(t.Context(), strings.NewReader("new.txt"))
		require.NoError(t, err)
		require.NoError(t, tmp.AddObjectToIndex(t.Context(), "120000", target, "data.bin"))
		tree, err := tmp.WriteTree(t.Context())
		require.NoError(t, err)
		sha, err = tmp.CommitTree(t.Context(), &files_service.CommitTreeUserOptions{
			ParentCommitID: sha,
			TreeHash:       tree,
			CommitMessage:  "replace data.bin with a symlink",
			DoerUser:       user2,
		})
		require.NoError(t, err)
		require.NoError(t, tmp.Push(t.Context(), user2, sha, "main", false))

		post("data.bin", "proposed", "1", http.StatusOK)
		post("data.bin", "previous", "1", http.StatusBadRequest)
	})
}
