// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
)

func TestRichDiff(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		resp := testCreateFileInBranch(t, user2, repo1, createFileInBranchOptions{}, map[string]string{
			"old.md":  "Hello old world",
			"new.md":  "Hello new world",
			"new.txt": "Hello new world",
		})
		link := func(oldPath, path string) string {
			q := url.Values{"before": {resp.Commit.SHA}, "old_path": {oldPath}, "after": {resp.Commit.SHA}, "path": {path}}
			return "/user2/repo1/rich_diff?" + q.Encode()
		}

		body := MakeRequest(t, NewRequest(t, "GET", link("old.md", "new.md")), http.StatusOK).Body.String()
		assert.Contains(t, body, `Hello <del class="rich-diff-removed">old</del><ins class="rich-diff-added">new</ins> world`)

		MakeRequest(t, NewRequest(t, "GET", link("old.md", "new.txt")), http.StatusNotFound)
		MakeRequest(t, NewRequest(t, "GET", link("new.txt", "new.md")), http.StatusNotFound)
	})
}
