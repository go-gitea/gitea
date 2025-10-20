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

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

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
				Name: util.ToPointer("approve-all-runs-fork"),
			}).AddTokenAuth(user4Token)
		resp := MakeRequest(t, req, http.StatusAccepted)
		var apiForkRepo api.Repository
		DecodeJSON(t, resp, &apiForkRepo)
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
		req = NewRequestWithValues(t, "POST", dataURL,
			map[string]string{
				"_csrf": GetUserCSRFToken(t, user2Session),
			})
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
