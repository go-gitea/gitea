// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestWorkflowWithInputsContext(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-inputs-context", false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		httpContext := NewAPITestContext(t, user2.Name, repo.Name, auth_model.AccessTokenScopeWriteRepository)
		defer doAPIDeleteRepository(httpContext)(t)

		wRunner := newMockRunner()
		wRunner.registerAsRepoRunner(t, user2.Name, repo.Name, "windows-runner", []string{"windows-runner"}, false)
		lRunner := newMockRunner()
		lRunner.registerAsRepoRunner(t, user2.Name, repo.Name, "linux-runner", []string{"linux-runner"}, false)

		wf1TreePath := ".gitea/workflows/test-inputs-context.yml"
		wf1FileContent := `name: Test Inputs Context
on:
  workflow_dispatch:
    inputs:
      os:
        description: 'OS'
        required: true
        type: choice
        options:
        - linux
        - windows

run-name: Build APP on ${{ inputs.os }}

jobs:
  build:
    runs-on: ${{ inputs.os }}-runner
    steps:
      - run: echo 'Start building APP'
`

		opts1 := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, "create %s"+wf1TreePath, wf1FileContent)
		createWorkflowFile(t, token, user2.Name, repo.Name, wf1TreePath, opts1)

		// run the workflow with os=windows
		urlStr := fmt.Sprintf("/%s/%s/actions/run?workflow=%s", user2.Name, repo.Name, "test-inputs-context.yml")
		req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
			"_csrf": GetUserCSRFToken(t, session),
			"ref":   "refs/heads/main",
			"os":    "windows",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		// linux-runner cannot fetch the task
		lRunner.fetchNoTask(t)

		task := wRunner.fetchTask(t)
		_, _, run := getTaskAndJobAndRunByTaskID(t, task.Id)
		assert.Equal(t, "Build APP on windows", run.Title)
	})
}

func getTaskAndJobAndRunByTaskID(t *testing.T, taskID int64) (*actions_model.ActionTask, *actions_model.ActionRunJob, *actions_model.ActionRun) {
	actionTask := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: taskID})
	actionRunJob := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: actionTask.JobID})
	actionRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: actionRunJob.RunID})
	return actionTask, actionRunJob, actionRun
}
