// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestRepoActivity(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		session := loginUser(t, "user1")

		// Create PRs (1 merged & 2 proposed)
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")
		resp := testPullCreate(t, session, "user1", "repo1", false, "master", "master", "This is a pull title")
		elem := strings.Split(test.RedirectURL(resp), "/")
		assert.EqualValues(t, "pulls", elem[3])
		testPullMerge(t, session, elem[1], elem[2], elem[4], repo_model.MergeStyleMerge, false)

		testEditFileToNewBranch(t, session, "user1", "repo1", "master", "feat/better_readme", "README.md", "Hello, World (Edited Again)\n")
		testPullCreate(t, session, "user1", "repo1", false, "master", "feat/better_readme", "This is a pull title")

		testEditFileToNewBranch(t, session, "user1", "repo1", "master", "feat/much_better_readme", "README.md", "Hello, World (Edited More)\n")
		testPullCreate(t, session, "user1", "repo1", false, "master", "feat/much_better_readme", "This is a pull title")

		// Create issues (3 new issues)
		testNewIssue(t, session, "user2", "repo1", "Issue 1", "Description 1")
		testNewIssue(t, session, "user2", "repo1", "Issue 2", "Description 2")
		testNewIssue(t, session, "user2", "repo1", "Issue 3", "Description 3")

		// Create releases (1 new release)
		createNewRelease(t, session, "/user2/repo1", "v1.0.0", "v1.0.0", false, false)

		// Open Activity page and check stats
		req := NewRequest(t, "GET", "/user2/repo1/activity")
		resp = session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)

		// Should be 1 published release
		list := htmlDoc.doc.Find("#published-releases").Next().Find("p.desc")
		assert.Len(t, list.Nodes, 1)

		// Should be 1 merged pull request
		list = htmlDoc.doc.Find("#merged-pull-requests").Next().Find("p.desc")
		assert.Len(t, list.Nodes, 1)

		// Should be 2 proposed pull requests
		list = htmlDoc.doc.Find("#proposed-pull-requests").Next().Find("p.desc")
		assert.Len(t, list.Nodes, 2)

		// Should be 3 new issues
		list = htmlDoc.doc.Find("#new-issues").Next().Find("p.desc")
		assert.Len(t, list.Nodes, 3)
	})
}
