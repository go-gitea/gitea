// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestActionsCollaborativeOwner(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		// user2 is the owner of the private "reusable_workflow" repo
		user2Session := loginUser(t, "user2")
		user2Token := getTokenForLoggedInUser(t, user2Session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
		apiReusableWorkflowRepo := createActionsTestRepo(t, user2Token, "reusable_workflow", true)
		reusableWorkflowRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiReusableWorkflowRepo.ID})

		// user4 is the owner of the private caller repo
		user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
		user4Session := loginUser(t, user4.Name)
		user4Token := getTokenForLoggedInUser(t, user4Session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
		apiCallerRepo := createActionsTestRepo(t, user4Token, "caller_workflow", true)
		callerRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiCallerRepo.ID})

		// create a mock runner for caller
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, callerRepo.OwnerName, callerRepo.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		// init the workflow for caller
		wfTreePath := ".gitea/workflows/test_collaborative_owner.yml"
		wfFileContent := `name: Test Collaborative Owner
on: push
jobs:
  job1:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'test collaborative owner'
`
		opts := getWorkflowCreateFileOptions(user4, callerRepo.DefaultBranch, "create "+wfTreePath, wfFileContent)
		createWorkflowFile(t, user4Token, callerRepo.OwnerName, callerRepo.Name, wfTreePath, opts)

		// fetch the task and get its token
		task := runner.fetchTask(t)
		taskToken := task.Secrets["GITEA_TOKEN"]
		assert.NotEmpty(t, taskToken)

		// prepare for clone
		dstPath := t.TempDir()
		u.Path = fmt.Sprintf("%s/%s.git", "user2", "reusable_workflow")
		u.User = url.UserPassword("gitea-actions", taskToken)

		// the git clone will fail
		doGitCloneFail(u)(t)

		// add user10 to the list of collaborative owners
		req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings/actions/general/collaborative_owner/add", reusableWorkflowRepo.OwnerName, reusableWorkflowRepo.Name), map[string]string{
			"collaborative_owner": user4.Name,
		})
		user2Session.MakeRequest(t, req, http.StatusOK)

		// the git clone will be successful
		doGitClone(dstPath, u)(t)

		// remove user10 from the list of collaborative owners
		req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/settings/actions/general/collaborative_owner/delete?id=%d", reusableWorkflowRepo.OwnerName, reusableWorkflowRepo.Name, user4.ID))
		user2Session.MakeRequest(t, req, http.StatusOK)

		// the git clone will fail
		doGitCloneFail(u)(t)
	})
}
