// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestPullCreate_CommitStatus(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1")
		testEditFileToNewBranch(t, session, "user1", "repo1", "master", "status1", "README.md", "status1")

		url := path.Join("user1", "repo1", "compare", "master...status1")
		req := NewRequestWithValues(t, "POST", url,
			map[string]string{
				"_csrf": GetCSRF(t, session, url),
				"title": "pull request from status1",
			},
		)
		session.MakeRequest(t, req, http.StatusOK)

		req = NewRequest(t, "GET", "/user1/repo1/pulls")
		resp := session.MakeRequest(t, req, http.StatusOK)
		NewHTMLParser(t, resp.Body)

		// Request repository commits page
		req = NewRequest(t, "GET", "/user1/repo1/pulls/1/commits")
		resp = session.MakeRequest(t, req, http.StatusOK)
		doc := NewHTMLParser(t, resp.Body)

		// Get first commit URL
		commitURL, exists := doc.doc.Find("#commits-table tbody tr td.sha a").Last().Attr("href")
		assert.True(t, exists)
		assert.NotEmpty(t, commitURL)

		commitID := path.Base(commitURL)

		statusList := []api.CommitStatusState{
			api.CommitStatusPending,
			api.CommitStatusError,
			api.CommitStatusFailure,
			api.CommitStatusSuccess,
			api.CommitStatusWarning,
		}

		statesIcons := map[api.CommitStatusState]string{
			api.CommitStatusPending: "octicon-dot-fill",
			api.CommitStatusSuccess: "octicon-check",
			api.CommitStatusError:   "gitea-exclamation",
			api.CommitStatusFailure: "octicon-x",
			api.CommitStatusWarning: "gitea-exclamation",
		}

		testCtx := NewAPITestContext(t, "user1", "repo1", auth_model.AccessTokenScopeWriteRepository)

		// Update commit status, and check if icon is updated as well
		for _, status := range statusList {

			// Call API to add status for commit
			t.Run("CreateStatus", doAPICreateCommitStatus(testCtx, commitID, api.CreateStatusOption{
				State:       status,
				TargetURL:   "http://test.ci/",
				Description: "",
				Context:     "testci",
			}))

			req = NewRequestf(t, "GET", "/user1/repo1/pulls/1/commits")
			resp = session.MakeRequest(t, req, http.StatusOK)
			doc = NewHTMLParser(t, resp.Body)

			commitURL, exists = doc.doc.Find("#commits-table tbody tr td.sha a").Last().Attr("href")
			assert.True(t, exists)
			assert.NotEmpty(t, commitURL)
			assert.EqualValues(t, commitID, path.Base(commitURL))

			cls, ok := doc.doc.Find("#commits-table tbody tr td.message .commit-status").Last().Attr("class")
			assert.True(t, ok)
			assert.Contains(t, cls, statesIcons[status])
		}
	})
}

func doAPICreateCommitStatus(ctx APITestContext, commitID string, data api.CreateStatusOption) func(*testing.T) {
	return func(t *testing.T) {
		req := NewRequestWithJSON(
			t,
			http.MethodPost,
			fmt.Sprintf("/api/v1/repos/%s/%s/statuses/%s?token=%s", ctx.Username, ctx.Reponame, commitID, ctx.Token),
			data,
		)
		if ctx.ExpectedCode != 0 {
			ctx.Session.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		ctx.Session.MakeRequest(t, req, http.StatusCreated)
	}
}

func TestPullCreate_EmptyChangesWithDifferentCommits(t *testing.T) {
	// Merge must continue if commits SHA are different, even if content is same
	// Reason: gitflow and merging master back into develop, where is high possibility, there are no changes
	// but just commit saying "Merge branch". And this meta commit can be also tagged,
	// so we need to have this meta commit also in develop branch.
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1")
		testEditFileToNewBranch(t, session, "user1", "repo1", "master", "status1", "README.md", "status1")
		testEditFileToNewBranch(t, session, "user1", "repo1", "status1", "status1", "README.md", "# repo1\n\nDescription for repo1")

		url := path.Join("user1", "repo1", "compare", "master...status1")
		req := NewRequestWithValues(t, "POST", url,
			map[string]string{
				"_csrf": GetCSRF(t, session, url),
				"title": "pull request from status1",
			},
		)
		session.MakeRequest(t, req, http.StatusOK)

		req = NewRequest(t, "GET", "/user1/repo1/pulls/1")
		resp := session.MakeRequest(t, req, http.StatusOK)
		doc := NewHTMLParser(t, resp.Body)

		text := strings.TrimSpace(doc.doc.Find(".merge-section").Text())
		assert.Contains(t, text, "This pull request can be merged automatically.")
	})
}

func TestPullCreate_EmptyChangesWithSameCommits(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1")
		testCreateBranch(t, session, "user1", "repo1", "branch/master", "status1", http.StatusSeeOther)
		url := path.Join("user1", "repo1", "compare", "master...status1")
		req := NewRequestWithValues(t, "POST", url,
			map[string]string{
				"_csrf": GetCSRF(t, session, url),
				"title": "pull request from status1",
			},
		)
		session.MakeRequest(t, req, http.StatusOK)
		req = NewRequest(t, "GET", "/user1/repo1/pulls/1")
		resp := session.MakeRequest(t, req, http.StatusOK)
		doc := NewHTMLParser(t, resp.Body)

		text := strings.TrimSpace(doc.doc.Find(".merge-section").Text())
		assert.Contains(t, text, "This branch is already included in the target branch. There is nothing to merge.")
	})
}
