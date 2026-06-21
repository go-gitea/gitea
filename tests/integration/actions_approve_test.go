// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	actions_model "gitea.dev/models/actions"
	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestApproveAllRunsOnPullRequestPage(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		// user2 is the owner of the base repo
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		user2Session := loginUser(t, user2.Name)
		user2Token := getTokenForLoggedInUser(t, user2Session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
		// user4 is the owner of the fork repo
		user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
		user4Session := loginUser(t, user4.Name)
		user4Token := getTokenForLoggedInUser(t, loginUser(t, user4.Name), auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiBaseRepo := createActionsTestRepo(t, user2Token, "approve-all-runs", false)
		baseRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiBaseRepo.ID})
		user2APICtx := NewAPITestContext(t, baseRepo.OwnerName, baseRepo.Name, auth_model.AccessTokenScopeWriteRepository)
		defer doAPIDeleteRepository(user2APICtx)(t)

		runner := newMockRunner()
		runner.registerAsRepoRunner(t, baseRepo.OwnerName, baseRepo.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		// init workflows
		wf1TreePath := ".gitea/workflows/pull_1.yml"
		wf1FileContent := `name: Pull 1
on: pull_request
jobs:
  unit-test:
    runs-on: ubuntu-latest
    steps:
      - run: echo unit-test
`
		opts1 := getWorkflowCreateFileOptions(user2, baseRepo.DefaultBranch, "create %s"+wf1TreePath, wf1FileContent)
		createWorkflowFile(t, user2Token, baseRepo.OwnerName, baseRepo.Name, wf1TreePath, opts1)
		wf2TreePath := ".gitea/workflows/pull_2.yml"
		wf2FileContent := `name: Pull 2
on: pull_request
jobs:
  integration-test:
    runs-on: ubuntu-latest
    steps:
      - run: echo integration-test
`
		opts2 := getWorkflowCreateFileOptions(user2, baseRepo.DefaultBranch, "create %s"+wf2TreePath, wf2FileContent)
		createWorkflowFile(t, user2Token, baseRepo.OwnerName, baseRepo.Name, wf2TreePath, opts2)

		// user4 forks the repo
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/forks", baseRepo.OwnerName, baseRepo.Name),
			&api.CreateForkOption{
				Name: new("approve-all-runs-fork"),
			}).AddTokenAuth(user4Token)
		resp := MakeRequest(t, req, http.StatusAccepted)
		apiForkRepo := DecodeJSON(t, resp, &api.Repository{})
		forkRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiForkRepo.ID})
		user4APICtx := NewAPITestContext(t, user4.Name, forkRepo.Name, auth_model.AccessTokenScopeWriteRepository)
		defer doAPIDeleteRepository(user4APICtx)(t)

		// user4 creates a pull request from branch "bugfix/user4"
		doAPICreateFile(user4APICtx, "user4-fix.txt", &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				NewBranchName: "bugfix/user4",
				Message:       "create user4-fix.txt",
				Author: api.Identity{
					Name:  user4.Name,
					Email: user4.Email,
				},
				Committer: api.Identity{
					Name:  user4.Name,
					Email: user4.Email,
				},
				Dates: api.CommitDateOptions{
					Author:    time.Now(),
					Committer: time.Now(),
				},
			},
			ContentBase64: base64.StdEncoding.EncodeToString([]byte("user4-fix")),
		})(t)
		apiPull, err := doAPICreatePullRequest(user4APICtx, baseRepo.OwnerName, baseRepo.Name, baseRepo.DefaultBranch, user4.Name+":bugfix/user4")(t)
		assert.NoError(t, err)

		// check runs
		run1 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: baseRepo.ID, TriggerUserID: user4.ID, WorkflowID: "pull_1.yml"})
		assert.True(t, run1.NeedApproval)
		assert.Equal(t, actions_model.StatusBlocked, run1.Status)
		run2 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: baseRepo.ID, TriggerUserID: user4.ID, WorkflowID: "pull_2.yml"})
		assert.True(t, run2.NeedApproval)
		assert.Equal(t, actions_model.StatusBlocked, run2.Status)

		// user4 cannot see the approve button
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/pulls/%d", baseRepo.OwnerName, baseRepo.Name, apiPull.Index))
		resp = user4Session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		assert.Zero(t, htmlDoc.doc.Find("#approve-status-checks button.link-action").Length())

		// user2 can see the approve button
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/pulls/%d", baseRepo.OwnerName, baseRepo.Name, apiPull.Index))
		resp = user2Session.MakeRequest(t, req, http.StatusOK)
		htmlDoc = NewHTMLParser(t, resp.Body)
		dataURL, exist := htmlDoc.doc.Find("#approve-status-checks button.link-action").Attr("data-url")
		assert.True(t, exist)
		assert.Equal(t,
			fmt.Sprintf("%s/actions/approve-all-checks?commit_id=%s",
				baseRepo.Link(), apiPull.Head.Sha),
			dataURL,
		)

		// user2 approves all runs
		req = NewRequest(t, "POST", dataURL)
		user2Session.MakeRequest(t, req, http.StatusOK)

		// check runs
		run1 = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run1.ID})
		assert.False(t, run1.NeedApproval)
		assert.Equal(t, user2.ID, run1.ApprovedBy)
		assert.Equal(t, actions_model.StatusWaiting, run1.Status)
		run2 = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run2.ID})
		assert.False(t, run2.NeedApproval)
		assert.Equal(t, user2.ID, run2.ApprovedBy)
		assert.Equal(t, actions_model.StatusWaiting, run2.Status)
	})
}

// TestForkPullRequestApprovalNotBypassedByPriorApproval verifies that a single
// approval on a fork PR does not permanently trust the contributor: a subsequent
// fork PR from the same user must still be gated (Blocked / NeedApproval=true)
// until that user has had a pull request merged in the repo.
func TestForkPullRequestApprovalNotBypassedByPriorApproval(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		user2Session := loginUser(t, user2.Name)
		user2Token := getTokenForLoggedInUser(t, user2Session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
		user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
		user4Session := loginUser(t, user4.Name)
		user4Token := getTokenForLoggedInUser(t, user4Session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiBaseRepo := createActionsTestRepo(t, user2Token, "fork-approval-regression", false)
		baseRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiBaseRepo.ID})
		user2APICtx := NewAPITestContext(t, baseRepo.OwnerName, baseRepo.Name, auth_model.AccessTokenScopeWriteRepository)
		defer doAPIDeleteRepository(user2APICtx)(t)

		wfTreePath := ".gitea/workflows/ci.yml"
		wfContent := `name: CI
on: pull_request
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: echo ok
`
		createWorkflowFile(t, user2Token, baseRepo.OwnerName, baseRepo.Name, wfTreePath,
			getWorkflowCreateFileOptions(user2, baseRepo.DefaultBranch, "add ci", wfContent))

		// user4 forks the repo
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/forks", baseRepo.OwnerName, baseRepo.Name),
			&api.CreateForkOption{Name: new("fork-approval-regression-fork")}).AddTokenAuth(user4Token)
		resp := MakeRequest(t, req, http.StatusAccepted)
		apiForkRepo := DecodeJSON(t, resp, &api.Repository{})
		forkRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiForkRepo.ID})
		user4APICtx := NewAPITestContext(t, user4.Name, forkRepo.Name, auth_model.AccessTokenScopeWriteRepository)
		defer doAPIDeleteRepository(user4APICtx)(t)

		// PR #1: a benign change from user4's fork — first-time contributor, gate engages.
		doAPICreateFile(user4APICtx, "first.txt", &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				NewBranchName: "first",
				Message:       "first",
				Author:        api.Identity{Name: user4.Name, Email: user4.Email},
				Committer:     api.Identity{Name: user4.Name, Email: user4.Email},
				Dates:         api.CommitDateOptions{Author: time.Now(), Committer: time.Now()},
			},
			ContentBase64: base64.StdEncoding.EncodeToString([]byte("first")),
		})(t)
		pr1, err := doAPICreatePullRequest(user4APICtx, baseRepo.OwnerName, baseRepo.Name, baseRepo.DefaultBranch, user4.Name+":first")(t)
		assert.NoError(t, err)

		run1 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: baseRepo.ID, TriggerUserID: user4.ID, Ref: fmt.Sprintf("refs/pull/%d/head", pr1.Index)})
		assert.True(t, run1.NeedApproval, "first fork PR must require approval")
		assert.Equal(t, actions_model.StatusBlocked, run1.Status)

		// user2 approves run1.
		req = NewRequest(t, "POST", fmt.Sprintf("%s/actions/approve-all-checks?commit_id=%s", baseRepo.Link(), pr1.Head.Sha))
		user2Session.MakeRequest(t, req, http.StatusOK)
		run1 = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run1.ID})
		assert.False(t, run1.NeedApproval)
		assert.Equal(t, user2.ID, run1.ApprovedBy)

		// PR #2: same user, fresh branch. Pre-fix, this run was created with
		// NeedApproval=false and dispatched immediately — the bypass path.
		doAPICreateFile(user4APICtx, "second.txt", &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				NewBranchName: "second",
				Message:       "second",
				Author:        api.Identity{Name: user4.Name, Email: user4.Email},
				Committer:     api.Identity{Name: user4.Name, Email: user4.Email},
				Dates:         api.CommitDateOptions{Author: time.Now(), Committer: time.Now()},
			},
			ContentBase64: base64.StdEncoding.EncodeToString([]byte("second")),
		})(t)
		pr2, err := doAPICreatePullRequest(user4APICtx, baseRepo.OwnerName, baseRepo.Name, baseRepo.DefaultBranch, user4.Name+":second")(t)
		assert.NoError(t, err)

		run2 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: baseRepo.ID, TriggerUserID: user4.ID, Ref: fmt.Sprintf("refs/pull/%d/head", pr2.Index)})
		assert.True(t, run2.NeedApproval, "second fork PR must still require approval — prior approval-to-run does not grant trust")
		assert.Equal(t, actions_model.StatusBlocked, run2.Status)
		assert.EqualValues(t, 0, run2.ApprovedBy)

		// After merging PR #1, user4 becomes a known contributor and the gate lifts for a new PR.
		doAPIMergePullRequest(user2APICtx, baseRepo.OwnerName, baseRepo.Name, pr1.Index)(t)
		doAPICreateFile(user4APICtx, "third.txt", &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				NewBranchName: "third",
				Message:       "third",
				Author:        api.Identity{Name: user4.Name, Email: user4.Email},
				Committer:     api.Identity{Name: user4.Name, Email: user4.Email},
				Dates:         api.CommitDateOptions{Author: time.Now(), Committer: time.Now()},
			},
			ContentBase64: base64.StdEncoding.EncodeToString([]byte("third")),
		})(t)
		pr3, err := doAPICreatePullRequest(user4APICtx, baseRepo.OwnerName, baseRepo.Name, baseRepo.DefaultBranch, user4.Name+":third")(t)
		assert.NoError(t, err)

		run3 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: baseRepo.ID, TriggerUserID: user4.ID, Ref: fmt.Sprintf("refs/pull/%d/head", pr3.Index)})
		assert.False(t, run3.NeedApproval, "fork PR from a user with a prior merged PR should not require approval")
	})
}
