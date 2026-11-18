// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommitComment(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	const commitSHA = "65f1bf27bc3bf70f64657658635e66094edbcb4d"
	commitURL := "/user2/repo1/commit/" + commitSHA
	poster := loginUser(t, "user4")
	other := loginUser(t, "user5")

	// the "+" button is rendered for signed-in users only
	resp := poster.MakeRequest(t, NewRequest(t, "GET", commitURL), http.StatusOK)
	assert.Contains(t, resp.Body.String(), "/user2/repo1/commit_comments/new/"+commitSHA)

	req := NewRequestWithValues(t, "POST", "/user2/repo1/commit_comments/create/"+commitSHA, map[string]string{
		"content": "inline commit comment",
		"side":    "proposed",
		"line":    "1",
		"path":    "README.md",
	})
	resp = poster.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "inline commit comment")
	comment := unittest.AssertExistsAndLoadBean(t, &git_model.CommitComment{RepoID: 1, PosterID: 4, TreePath: "README.md"})
	assert.Equal(t, commitSHA, comment.CommitSHA)
	assert.Equal(t, int64(1), comment.Line)

	attach := &repo_model.Attachment{UUID: "commit-comment-attach", RepoID: 1, UploaderID: 4, Name: "a.txt", CommitCommentID: comment.ID}
	require.NoError(t, db.Insert(t.Context(), attach))

	resp = poster.MakeRequest(t, NewRequest(t, "GET", commitURL), http.StatusOK)
	assert.Contains(t, resp.Body.String(), "inline commit comment")
	assert.Contains(t, resp.Body.String(), "a.txt")

	commentURL := fmt.Sprintf("/user2/repo1/commit_comments/%d", comment.ID)

	// other users can't change the comment
	req = NewRequestWithValues(t, "POST", commentURL+"/edit", map[string]string{"content": "hijacked", "content_version": "0"})
	other.MakeRequest(t, req, http.StatusForbidden)
	req = NewRequest(t, "POST", commentURL+"/delete")
	other.MakeRequest(t, req, http.StatusForbidden)

	// edit without any "files[]" drops the attachment
	req = NewRequestWithValues(t, "POST", commentURL+"/edit", map[string]string{"content": "edited", "content_version": "0"})
	poster.MakeRequest(t, req, http.StatusOK)
	comment = unittest.AssertExistsAndLoadBean(t, &git_model.CommitComment{ID: comment.ID})
	assert.Equal(t, "edited", comment.Content)
	assert.Equal(t, 1, comment.ContentVersion)
	unittest.AssertNotExistsBean(t, &repo_model.Attachment{UUID: attach.UUID})

	// stale content version is rejected
	req = NewRequestWithValues(t, "POST", commentURL+"/edit", map[string]string{"content": "stale", "content_version": "0"})
	poster.MakeRequest(t, req, http.StatusBadRequest)

	req = NewRequest(t, "POST", commentURL+"/delete")
	poster.MakeRequest(t, req, http.StatusOK)
	unittest.AssertNotExistsBean(t, &git_model.CommitComment{ID: comment.ID})
}
