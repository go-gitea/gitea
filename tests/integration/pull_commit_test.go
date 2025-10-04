// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/commitstatus"
	api "code.gitea.io/gitea/modules/structs"
	pull_service "code.gitea.io/gitea/services/pull"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListPullCommits(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user5")
	req := NewRequest(t, "GET", "/user2/repo1/pulls/3/commits/list")
	resp := session.MakeRequest(t, req, http.StatusOK)

	var pullCommitList struct {
		Commits             []pull_service.CommitInfo `json:"commits"`
		LastReviewCommitSha string                    `json:"last_review_commit_sha"`
	}
	DecodeJSON(t, resp, &pullCommitList)

	require.Len(t, pullCommitList.Commits, 2)
	assert.Equal(t, "985f0301dba5e7b34be866819cd15ad3d8f508ee", pullCommitList.Commits[0].ID)
	assert.Equal(t, "5c050d3b6d2db231ab1f64e324f1b6b9a0b181c2", pullCommitList.Commits[1].ID)
	assert.Equal(t, "4a357436d925b5c974181ff12a994538ddc5a269", pullCommitList.LastReviewCommitSha)

	t.Run("CommitBlobExcerpt", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req = NewRequest(t, "GET", "/user2/repo1/blob_excerpt/985f0301dba5e7b34be866819cd15ad3d8f508ee?last_left=0&last_right=0&left=2&right=2&left_hunk_size=2&right_hunk_size=2&path=README.md&style=split&direction=up")
		resp = session.MakeRequest(t, req, http.StatusOK)
		assert.Contains(t, resp.Body.String(), `<td class="lines-code lines-code-new"><code class="code-inner"># repo1</code>`)
	})
}

func TestPullCreate_CommitStatus(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFileToNewBranch(t, session, "user1", "repo1", "master", "status1", "README.md", "status1")

		url := path.Join("user1", "repo1", "compare", "master...status1")
		req := NewRequestWithValues(t, "POST", url,
			map[string]string{
				"_csrf": GetUserCSRFToken(t, session),
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

		statusList := []commitstatus.CommitStatusState{
			commitstatus.CommitStatusPending,
			commitstatus.CommitStatusError,
			commitstatus.CommitStatusFailure,
			commitstatus.CommitStatusSuccess,
			commitstatus.CommitStatusWarning,
		}

		statesIcons := map[commitstatus.CommitStatusState]string{
			commitstatus.CommitStatusPending: "octicon-dot-fill",
			commitstatus.CommitStatusSuccess: "octicon-check",
			commitstatus.CommitStatusError:   "gitea-exclamation",
			commitstatus.CommitStatusFailure: "octicon-x",
			commitstatus.CommitStatusWarning: "gitea-exclamation",
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

			req = NewRequest(t, "GET", "/user1/repo1/pulls/1/commits")
			resp = session.MakeRequest(t, req, http.StatusOK)
			doc = NewHTMLParser(t, resp.Body)

			commitURL, exists = doc.doc.Find("#commits-table tbody tr td.sha a").Last().Attr("href")
			assert.True(t, exists)
			assert.NotEmpty(t, commitURL)
			assert.Equal(t, commitID, path.Base(commitURL))

			cls, ok := doc.doc.Find("#commits-table tbody tr td.message .commit-status").Last().Attr("class")
			assert.True(t, ok)
			assert.Contains(t, cls, statesIcons[status])
		}

		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user1", Name: "repo1"})
		css := unittest.AssertExistsAndLoadBean(t, &git_model.CommitStatusSummary{RepoID: repo1.ID, SHA: commitID})
		assert.Equal(t, commitstatus.CommitStatusSuccess, css.State)
	})
}

func doAPICreateCommitStatus(ctx APITestContext, commitID string, data api.CreateStatusOption) func(*testing.T) {
	return func(t *testing.T) {
		req := NewRequestWithJSON(
			t,
			http.MethodPost,
			fmt.Sprintf("/api/v1/repos/%s/%s/statuses/%s", ctx.Username, ctx.Reponame, commitID),
			data,
		).AddTokenAuth(ctx.Token)
		if ctx.ExpectedCode != 0 {
			ctx.Session.MakeRequest(t, req, ctx.ExpectedCode)
			return
		}
		ctx.Session.MakeRequest(t, req, http.StatusCreated)
	}
}
