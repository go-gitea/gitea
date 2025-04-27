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
	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/services/pull"

	"github.com/stretchr/testify/assert"
)

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
		assert.Equal(t, api.CommitStatusWarning, css.State)
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

func TestPullCreate_EmptyChangesWithDifferentCommits(t *testing.T) {
	// Merge must continue if commits SHA are different, even if content is same
	// Reason: gitflow and merging master back into develop, where is high possibility, there are no changes
	// but just commit saying "Merge branch". And this meta commit can be also tagged,
	// so we need to have this meta commit also in develop branch.
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testEditFileToNewBranch(t, session, "user1", "repo1", "master", "status1", "README.md", "status1")
		testEditFileToNewBranch(t, session, "user1", "repo1", "status1", "status1", "README.md", "# repo1\n\nDescription for repo1")

		url := path.Join("user1", "repo1", "compare", "master...status1")
		req := NewRequestWithValues(t, "POST", url,
			map[string]string{
				"_csrf": GetUserCSRFToken(t, session),
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
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1", "")
		testCreateBranch(t, session, "user1", "repo1", "branch/master", "status1", http.StatusSeeOther)
		url := path.Join("user1", "repo1", "compare", "master...status1")
		req := NewRequestWithValues(t, "POST", url,
			map[string]string{
				"_csrf": GetUserCSRFToken(t, session),
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

func TestPullStatusDelayCheck(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		defer test.MockVariableValue(&setting.Repository.PullRequest.DelayCheckForInactiveDays, 1)()
		defer test.MockVariableValue(&pull.AddPullRequestToCheckQueue)()

		session := loginUser(t, "user2")

		run := func(t *testing.T, fn func(*testing.T)) (issue3 *issues.Issue, checkedPrID int64) {
			pull.AddPullRequestToCheckQueue = func(prID int64) {
				checkedPrID = prID
			}
			fn(t)
			issue3 = unittest.AssertExistsAndLoadBean(t, &issues.Issue{RepoID: 1, Index: 3})
			_ = issue3.LoadPullRequest(t.Context())
			return issue3, checkedPrID
		}

		assertReloadingInterval := func(t *testing.T, interval string) {
			req := NewRequest(t, "GET", "/user2/repo1/pulls/3")
			resp := session.MakeRequest(t, req, http.StatusOK)
			attr := "data-pull-merge-box-reloading-interval"
			if interval == "" {
				assert.NotContains(t, resp.Body.String(), attr)
			} else {
				assert.Contains(t, resp.Body.String(), fmt.Sprintf(`%s="%v"`, attr, interval))
			}
		}

		// PR issue3 is merageable at the beginning
		issue3, checkedPrID := run(t, func(t *testing.T) {})
		assert.Equal(t, issues.PullRequestStatusMergeable, issue3.PullRequest.Status)
		assert.Zero(t, checkedPrID)
		assertReloadingInterval(t, "") // the PR is mergeable, so no need to reload the merge box

		// setting.IsProd = false // it would cause data-race because the queue handlers might be running and reading its value
		// assertReloadingInterval(t, "1") // make sure dev mode always do merge box reloading, to make sure the UI logic won't break
		// setting.IsProd = true

		// when base branch changes, PR status should be updated, but it is inactive for long time, so no real check
		issue3, checkedPrID = run(t, func(t *testing.T) {
			testEditFile(t, session, "user2", "repo1", "master", "README.md", "new content 1")
		})
		assert.Equal(t, issues.PullRequestStatusChecking, issue3.PullRequest.Status)
		assert.Zero(t, checkedPrID)
		assertReloadingInterval(t, "2000") // the PR status is "checking", so try to reload the merge box

		// view a PR with status=checking, it starts the real check
		issue3, checkedPrID = run(t, func(t *testing.T) {
			req := NewRequest(t, "GET", "/user2/repo1/pulls/3")
			session.MakeRequest(t, req, http.StatusOK)
		})
		assert.Equal(t, issues.PullRequestStatusChecking, issue3.PullRequest.Status)
		assert.Equal(t, issue3.PullRequest.ID, checkedPrID)

		// when base branch changes, still so no real check
		issue3, checkedPrID = run(t, func(t *testing.T) {
			testEditFile(t, session, "user2", "repo1", "master", "README.md", "new content 2")
		})
		assert.Equal(t, issues.PullRequestStatusChecking, issue3.PullRequest.Status)
		assert.Zero(t, checkedPrID)

		// then allow to check PRs without delay, when base branch changes, the PRs will be checked
		setting.Repository.PullRequest.DelayCheckForInactiveDays = -1
		issue3, checkedPrID = run(t, func(t *testing.T) {
			testEditFile(t, session, "user2", "repo1", "master", "README.md", "new content 3")
		})
		assert.Equal(t, issues.PullRequestStatusChecking, issue3.PullRequest.Status)
		assert.Equal(t, issue3.PullRequest.ID, checkedPrID)
	})
}
