// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	webhook_module "code.gitea.io/gitea/modules/webhook"
	actions_service "code.gitea.io/gitea/services/actions"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"github.com/stretchr/testify/assert"
)

func TestWorkflowConcurrency(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-concurrency", false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		httpContext := NewAPITestContext(t, user2.Name, repo.Name, auth_model.AccessTokenScopeWriteRepository)
		defer doAPIDeleteRepository(httpContext)(t)

		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, repo.Name, "mock-runner", []string{"ubuntu-latest"})

		// add a variable for test
		req := NewRequestWithJSON(t, "POST",
			fmt.Sprintf("/api/v1/repos/%s/%s/actions/variables/myvar", user2.Name, repo.Name), &api.CreateVariableOption{
				Value: "abc123",
			}).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		wf1TreePath := ".gitea/workflows/concurrent-workflow-1.yml"
		wf1FileContent := `name: concurrent-workflow-1
on: 
  push:
    paths:
      - '.gitea/workflows/concurrent-workflow-1.yml'
concurrency:
  group: workflow-main-abc123-user2
jobs:
  wf1-job:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'job from workflow1'
`
		wf2TreePath := ".gitea/workflows/concurrent-workflow-2.yml"
		wf2FileContent := `name: concurrent-workflow-2
on: 
  push:
    paths:
      - '.gitea/workflows/concurrent-workflow-2.yml'
concurrency:
  group: workflow-${{ gitea.ref_name }}-${{ vars.myvar }}-${{ gitea.event.pusher.username }}
jobs:
  wf2-job:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'job from workflow2'
`
		wf3TreePath := ".gitea/workflows/concurrent-workflow-3.yml"
		wf3FileContent := `name: concurrent-workflow-3
on: 
  push:
    paths:
      - '.gitea/workflows/concurrent-workflow-3.yml'
concurrency:
  group: workflow-main-abc${{ 123 }}-${{ gitea.event.pusher.username }}
jobs:
  wf3-job:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'job from workflow3'
`
		opts1 := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, fmt.Sprintf("create %s", wf1TreePath), wf1FileContent)
		createWorkflowFile(t, token, user2.Name, repo.Name, wf1TreePath, opts1)
		opts2 := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, fmt.Sprintf("create %s", wf2TreePath), wf2FileContent)
		createWorkflowFile(t, token, user2.Name, repo.Name, wf2TreePath, opts2)
		opts3 := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, fmt.Sprintf("create %s", wf3TreePath), wf3FileContent)
		createWorkflowFile(t, token, user2.Name, repo.Name, wf3TreePath, opts3)

		// fetch and exec workflow1, workflow2 and workflow3 are blocked
		task := runner.fetchTask(t)
		_, _, run := getTaskAndJobAndRunByTaskID(t, task.Id)
		assert.Equal(t, "workflow-main-abc123-user2", run.ConcurrencyGroup)
		assert.Equal(t, "concurrent-workflow-1.yml", run.WorkflowID)
		runner.fetchNoTask(t)
		runner.execTask(t, task, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})

		// fetch workflow2 or workflow3
		workflowNames := []string{"concurrent-workflow-2.yml", "concurrent-workflow-3.yml"}
		task = runner.fetchTask(t)
		_, _, run = getTaskAndJobAndRunByTaskID(t, task.Id)
		assert.Contains(t, workflowNames, run.WorkflowID)
		workflowNames = slices.DeleteFunc(workflowNames, func(wfn string) bool { return wfn == run.WorkflowID })
		assert.Equal(t, "workflow-main-abc123-user2", run.ConcurrencyGroup)
		runner.fetchNoTask(t)
		runner.execTask(t, task, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})

		// fetch the last workflow (workflow2 or workflow3)
		task = runner.fetchTask(t)
		_, _, run = getTaskAndJobAndRunByTaskID(t, task.Id)
		assert.Equal(t, "workflow-main-abc123-user2", run.ConcurrencyGroup)
		assert.Equal(t, workflowNames[0], run.WorkflowID)
		runner.fetchNoTask(t)
		runner.execTask(t, task, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})
	})
}

func TestPullRequestWorkflowConcurrency(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		// user2 is the owner of the base repo
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		user2Session := loginUser(t, user2.Name)
		user2Token := getTokenForLoggedInUser(t, user2Session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
		// user4 is the owner of the forked repo
		user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
		user4Token := getTokenForLoggedInUser(t, loginUser(t, user4.Name), auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiBaseRepo := createActionsTestRepo(t, user2Token, "actions-concurrency", false)
		baseRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiBaseRepo.ID})
		user2APICtx := NewAPITestContext(t, baseRepo.OwnerName, baseRepo.Name, auth_model.AccessTokenScopeWriteRepository)
		defer doAPIDeleteRepository(user2APICtx)(t)

		runner := newMockRunner()
		runner.registerAsRepoRunner(t, baseRepo.OwnerName, baseRepo.Name, "mock-runner", []string{"ubuntu-latest"})

		// init the workflow
		wfTreePath := ".gitea/workflows/pull.yml"
		wfFileContent := `name: Pull Request
on: pull_request
concurrency:
  group: pull-request-test
  cancel-in-progress: ${{ !startsWith(gitea.head_ref, 'do-not-cancel/') }}
jobs:
  wf1-job:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'test the pull'
`
		opts1 := getWorkflowCreateFileOptions(user2, baseRepo.DefaultBranch, fmt.Sprintf("create %s", wfTreePath), wfFileContent)
		createWorkflowFile(t, user2Token, baseRepo.OwnerName, baseRepo.Name, wfTreePath, opts1)
		// user2 creates a pull request
		doAPICreateFile(user2APICtx, "user2-fix.txt", &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				NewBranchName: "bugfix/aaa",
				Message:       "create user2-fix.txt",
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
			ContentBase64: base64.StdEncoding.EncodeToString([]byte("user2-fix")),
		})(t)
		doAPICreatePullRequest(user2APICtx, baseRepo.OwnerName, baseRepo.Name, baseRepo.DefaultBranch, "bugfix/aaa")(t)
		pr1Task1 := runner.fetchTask(t)
		_, _, pr1Run1 := getTaskAndJobAndRunByTaskID(t, pr1Task1.Id)
		assert.Equal(t, "pull-request-test", pr1Run1.ConcurrencyGroup)
		assert.True(t, pr1Run1.ConcurrencyCancel)
		assert.Equal(t, actions_model.StatusRunning, pr1Run1.Status)

		// user4 forks the repo
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/forks", baseRepo.OwnerName, baseRepo.Name),
			&api.CreateForkOption{
				Name: util.ToPointer("actions-concurrency-fork"),
			}).AddTokenAuth(user4Token)
		resp := MakeRequest(t, req, http.StatusAccepted)
		var apiForkRepo api.Repository
		DecodeJSON(t, resp, &apiForkRepo)
		forkRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiForkRepo.ID})
		user4APICtx := NewAPITestContext(t, user4.Name, forkRepo.Name, auth_model.AccessTokenScopeWriteRepository)
		defer doAPIDeleteRepository(user4APICtx)(t)

		// user4 creates a pull request from branch "bugfix/bbb"
		doAPICreateFile(user4APICtx, "user4-fix.txt", &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				NewBranchName: "bugfix/bbb",
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
		doAPICreatePullRequest(user4APICtx, baseRepo.OwnerName, baseRepo.Name, baseRepo.DefaultBranch, fmt.Sprintf("%s:bugfix/bbb", user4.Name))(t)
		// cannot fetch the task because an approval is required
		runner.fetchNoTask(t)
		// user2 approves the run
		pr2Run1 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: baseRepo.ID, TriggerUserID: user4.ID})
		req = NewRequestWithValues(t, "POST",
			fmt.Sprintf("/%s/%s/actions/runs/%d/approve", baseRepo.OwnerName, baseRepo.Name, pr2Run1.Index),
			map[string]string{
				"_csrf": GetUserCSRFToken(t, user2Session),
			})
		user2Session.MakeRequest(t, req, http.StatusOK)
		// fetch the task and the previous task has been cancelled
		pr2Task1 := runner.fetchTask(t)
		_, _, pr2Run1 = getTaskAndJobAndRunByTaskID(t, pr2Task1.Id)
		assert.Equal(t, "pull-request-test", pr2Run1.ConcurrencyGroup)
		assert.True(t, pr2Run1.ConcurrencyCancel)
		assert.Equal(t, actions_model.StatusRunning, pr2Run1.Status)
		pr1Run1 = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: pr1Run1.ID})
		assert.Equal(t, actions_model.StatusCancelled, pr1Run1.Status)

		// user4 creates another pull request from branch "do-not-cancel/ccc"
		doAPICreateFile(user4APICtx, "user4-fix2.txt", &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				NewBranchName: "do-not-cancel/ccc",
				Message:       "create user4-fix2.txt",
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
			ContentBase64: base64.StdEncoding.EncodeToString([]byte("user4-fix2")),
		})(t)
		doAPICreatePullRequest(user4APICtx, baseRepo.OwnerName, baseRepo.Name, baseRepo.DefaultBranch, fmt.Sprintf("%s:do-not-cancel/ccc", user4.Name))(t)
		// cannot fetch the task because cancel-in-progress is false
		runner.fetchNoTask(t)
		runner.execTask(t, pr2Task1, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})
		pr2Run1 = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: pr2Run1.ID})
		assert.Equal(t, actions_model.StatusSuccess, pr2Run1.Status)
		// fetch the task
		pr3Task1 := runner.fetchTask(t)
		_, _, pr3Run1 := getTaskAndJobAndRunByTaskID(t, pr3Task1.Id)
		assert.Equal(t, "pull-request-test", pr3Run1.ConcurrencyGroup)
		assert.False(t, pr3Run1.ConcurrencyCancel)
		assert.Equal(t, actions_model.StatusRunning, pr3Run1.Status)
	})
}

func TestJobConcurrency(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-concurrency", false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		httpContext := NewAPITestContext(t, user2.Name, repo.Name, auth_model.AccessTokenScopeWriteRepository)
		defer doAPIDeleteRepository(httpContext)(t)

		runner1 := newMockRunner()
		runner1.registerAsRepoRunner(t, user2.Name, repo.Name, "mock-runner-1", []string{"runner1"})
		runner2 := newMockRunner()
		runner2.registerAsRepoRunner(t, user2.Name, repo.Name, "mock-runner-2", []string{"runner2"})

		// add a variable for test
		req := NewRequestWithJSON(t, "POST",
			fmt.Sprintf("/api/v1/repos/%s/%s/actions/variables/version_var", user2.Name, repo.Name), &api.CreateVariableOption{
				Value: "v1.23.0",
			}).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		wf1TreePath := ".gitea/workflows/concurrent-workflow-1.yml"
		wf1FileContent := `name: concurrent-workflow-1
on: 
  push:
    paths:
      - '.gitea/workflows/concurrent-workflow-1.yml'
jobs:
  wf1-job1:
    runs-on: runner1
    concurrency:
      group: job-main-${{ vars.version_var }}
    steps:
      - run: echo 'wf1-job1'
`
		wf2TreePath := ".gitea/workflows/concurrent-workflow-2.yml"
		wf2FileContent := `name: concurrent-workflow-2
on: 
  push:
    paths:
      - '.gitea/workflows/concurrent-workflow-2.yml'
jobs:
  wf2-job1:
    runs-on: runner2
    outputs:
      version: ${{ steps.version_step.outputs.app_version }} 
    steps:
      - id: version_step
        run: echo "app_version=v1.23.0" >> "$GITHUB_OUTPUT"
  wf2-job2:
    runs-on: runner1
    needs: [wf2-job1]
    concurrency:
      group: job-main-${{ needs.wf2-job1.outputs.version }}
    steps:
      - run: echo 'wf2-job2'
`
		wf3TreePath := ".gitea/workflows/concurrent-workflow-3.yml"
		wf3FileContent := `name: concurrent-workflow-3
on: 
  push:
    paths:
      - '.gitea/workflows/concurrent-workflow-3.yml'
jobs:
  wf3-job1:
    runs-on: runner1
    concurrency:
      group: job-main-${{ vars.version_var }}
      cancel-in-progress: ${{ vars.version_var == 'v1.23.0' }}
    steps:
      - run: echo 'wf3-job1'
`

		opts1 := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, fmt.Sprintf("create %s", wf1TreePath), wf1FileContent)
		createWorkflowFile(t, token, user2.Name, repo.Name, wf1TreePath, opts1)
		opts2 := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, fmt.Sprintf("create %s", wf2TreePath), wf2FileContent)
		createWorkflowFile(t, token, user2.Name, repo.Name, wf2TreePath, opts2)

		// fetch wf1-job1
		wf1Job1Task := runner1.fetchTask(t)
		_, wf1Job1ActionJob, _ := getTaskAndJobAndRunByTaskID(t, wf1Job1Task.Id)
		assert.Equal(t, "job-main-v1.23.0", wf1Job1ActionJob.ConcurrencyGroup)
		assert.Equal(t, actions_model.StatusRunning, wf1Job1ActionJob.Status)
		// fetch and exec wf2-job1
		wf2Job1Task := runner2.fetchTask(t)
		runner2.execTask(t, wf2Job1Task, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
			outputs: map[string]string{
				"version": "v1.23.0",
			},
		})
		// cannot fetch wf2-job2 because wf1-job1 is running
		runner1.fetchNoTask(t)
		// exec wf1-job1
		runner1.execTask(t, wf1Job1Task, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})
		// fetch wf2-job2
		wf2Job2Task := runner1.fetchTask(t)
		_, wf2Job2ActionJob, _ := getTaskAndJobAndRunByTaskID(t, wf2Job2Task.Id)
		assert.Equal(t, "job-main-v1.23.0", wf2Job2ActionJob.ConcurrencyGroup)
		assert.Equal(t, actions_model.StatusRunning, wf2Job2ActionJob.Status)
		// push workflow3 to trigger wf3-job1
		opts3 := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, fmt.Sprintf("create %s", wf3TreePath), wf3FileContent)
		createWorkflowFile(t, token, user2.Name, repo.Name, wf3TreePath, opts3)
		// fetch wf3-job1
		wf3Job1Task := runner1.fetchTask(t)
		_, wf3Job1ActionJob, _ := getTaskAndJobAndRunByTaskID(t, wf3Job1Task.Id)
		assert.Equal(t, "job-main-v1.23.0", wf3Job1ActionJob.ConcurrencyGroup)
		assert.Equal(t, actions_model.StatusRunning, wf3Job1ActionJob.Status)
		// wf2-job2 has been cancelled
		_, wf2Job2ActionJob, _ = getTaskAndJobAndRunByTaskID(t, wf2Job2Task.Id)
		assert.Equal(t, actions_model.StatusCancelled, wf2Job2ActionJob.Status)
	})
}

func TestMatrixConcurrency(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-concurrency", false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		httpContext := NewAPITestContext(t, user2.Name, repo.Name, auth_model.AccessTokenScopeWriteRepository)
		defer doAPIDeleteRepository(httpContext)(t)

		linuxRunner := newMockRunner()
		linuxRunner.registerAsRepoRunner(t, user2.Name, repo.Name, "mock-linux-runner", []string{"linux-runner"})
		windowsRunner := newMockRunner()
		windowsRunner.registerAsRepoRunner(t, user2.Name, repo.Name, "mock-windows-runner", []string{"windows-runner"})
		darwinRunner := newMockRunner()
		darwinRunner.registerAsRepoRunner(t, user2.Name, repo.Name, "mock-darwin-runner", []string{"darwin-runner"})

		wf1TreePath := ".gitea/workflows/concurrent-workflow-1.yml"
		wf1FileContent := `name: concurrent-workflow-1
on: 
  push:
    paths:
      - '.gitea/workflows/concurrent-workflow-1.yml'
jobs:
  job1:
    runs-on: ${{ matrix.os }}-runner
    strategy:
      matrix:
        os: [windows, linux]
    concurrency:
      group: job-os-${{ matrix.os }}
    steps:
      - run: echo 'job1'
  job2:
    runs-on: ${{ matrix.os }}-runner
    strategy:
      matrix:
        os: [darwin, windows, linux]
    concurrency:
      group: job-os-${{ matrix.os }}
    steps:
      - run: echo 'job2'
`

		opts1 := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, fmt.Sprintf("create %s", wf1TreePath), wf1FileContent)
		createWorkflowFile(t, token, user2.Name, repo.Name, wf1TreePath, opts1)

		job1WinTask := windowsRunner.fetchTask(t)
		job1LinuxTask := linuxRunner.fetchTask(t)
		windowsRunner.fetchNoTask(t)
		linuxRunner.fetchNoTask(t)
		job2DarwinTask := darwinRunner.fetchTask(t)
		_, job1WinJob, _ := getTaskAndJobAndRunByTaskID(t, job1WinTask.Id)
		assert.Equal(t, "job1 (windows)", job1WinJob.Name)
		assert.Equal(t, "job-os-windows", job1WinJob.ConcurrencyGroup)
		_, job1LinuxJob, _ := getTaskAndJobAndRunByTaskID(t, job1LinuxTask.Id)
		assert.Equal(t, "job1 (linux)", job1LinuxJob.Name)
		assert.Equal(t, "job-os-linux", job1LinuxJob.ConcurrencyGroup)
		_, job2DarwinJob, _ := getTaskAndJobAndRunByTaskID(t, job2DarwinTask.Id)
		assert.Equal(t, "job2 (darwin)", job2DarwinJob.Name)
		assert.Equal(t, "job-os-darwin", job2DarwinJob.ConcurrencyGroup)
		windowsRunner.execTask(t, job1WinTask, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})
		linuxRunner.execTask(t, job1LinuxTask, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})

		job2WinTask := windowsRunner.fetchTask(t)
		job2LinuxTask := linuxRunner.fetchTask(t)
		_, job2WinJob, _ := getTaskAndJobAndRunByTaskID(t, job2WinTask.Id)
		assert.Equal(t, "job2 (windows)", job2WinJob.Name)
		assert.Equal(t, "job-os-windows", job2WinJob.ConcurrencyGroup)
		_, job2LinuxJob, _ := getTaskAndJobAndRunByTaskID(t, job2LinuxTask.Id)
		assert.Equal(t, "job2 (linux)", job2LinuxJob.Name)
		assert.Equal(t, "job-os-linux", job2LinuxJob.ConcurrencyGroup)
	})
}

func TestWorkflowDispatchConcurrency(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-concurrency", false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		httpContext := NewAPITestContext(t, user2.Name, repo.Name, auth_model.AccessTokenScopeWriteRepository)
		defer doAPIDeleteRepository(httpContext)(t)

		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, repo.Name, "mock-runner", []string{"ubuntu-latest"})

		wf1TreePath := ".gitea/workflows/workflow-dispatch-concurrency.yml"
		wf1FileContent := `name: workflow-dispatch-concurrency
on:
  workflow_dispatch:
    inputs:
      appVersion:
        description: 'APP version'
        required: true
        default: 'v1.23'
        type: choice
        options:
        - v1.21
        - v1.22
        - v1.23
      cancel:
        description: 'Cancel running workflows'
        required: false
        type: boolean
        default: false
concurrency:
  group: workflow-dispatch-${{ inputs.appVersion }}
  cancel-in-progress: ${{ inputs.cancel }}
jobs:
  job:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'workflow dispatch job'
`

		opts1 := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, fmt.Sprintf("create %s", wf1TreePath), wf1FileContent)
		createWorkflowFile(t, token, user2.Name, repo.Name, wf1TreePath, opts1)

		// run the workflow with appVersion=v1.21 and cancel=false
		urlStr := fmt.Sprintf("/%s/%s/actions/run?workflow=%s", user2.Name, repo.Name, "workflow-dispatch-concurrency.yml")
		req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
			"_csrf":      GetUserCSRFToken(t, session),
			"ref":        "refs/heads/main",
			"appVersion": "v1.21",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)
		task1 := runner.fetchTask(t)
		_, _, run1 := getTaskAndJobAndRunByTaskID(t, task1.Id)
		assert.Equal(t, "workflow-dispatch-v1.21", run1.ConcurrencyGroup)

		// run the workflow with appVersion=v1.22 and cancel=false
		req = NewRequestWithValues(t, "POST", urlStr, map[string]string{
			"_csrf":      GetUserCSRFToken(t, session),
			"ref":        "refs/heads/main",
			"appVersion": "v1.22",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)
		task2 := runner.fetchTask(t)
		_, _, run2 := getTaskAndJobAndRunByTaskID(t, task2.Id)
		assert.Equal(t, "workflow-dispatch-v1.22", run2.ConcurrencyGroup)

		// run the workflow with appVersion=v1.22 and cancel=false again
		req = NewRequestWithValues(t, "POST", urlStr, map[string]string{
			"_csrf":      GetUserCSRFToken(t, session),
			"ref":        "refs/heads/main",
			"appVersion": "v1.22",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)
		runner.fetchNoTask(t) // cannot fetch task because task2 is not completed

		// run the workflow with appVersion=v1.22 and cancel=true
		req = NewRequestWithValues(t, "POST", urlStr, map[string]string{
			"_csrf":      GetUserCSRFToken(t, session),
			"ref":        "refs/heads/main",
			"appVersion": "v1.22",
			"cancel":     "on",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)
		task4 := runner.fetchTask(t)
		_, _, run4 := getTaskAndJobAndRunByTaskID(t, task4.Id)
		assert.Equal(t, "workflow-dispatch-v1.22", run4.ConcurrencyGroup)
		_, _, run2 = getTaskAndJobAndRunByTaskID(t, task2.Id)
		assert.Equal(t, actions_model.StatusCancelled, run2.Status)
	})
}

func TestScheduleConcurrency(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-concurrency", false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		httpContext := NewAPITestContext(t, user2.Name, repo.Name, auth_model.AccessTokenScopeWriteRepository)
		defer doAPIDeleteRepository(httpContext)(t)

		runner := newMockRunner()
		runner.registerAsRepoRunner(t, user2.Name, repo.Name, "mock-runner", []string{"ubuntu-latest"})

		wf1TreePath := ".gitea/workflows/schedule-concurrency.yml"
		wf1FileContent := `name: schedule-concurrency
on:
  push:
  schedule:
    - cron:  '@every 1m'
concurrency:
  group: schedule-concurrency
  cancel-in-progress: ${{ gitea.event_name == 'push' }}
jobs:
  job:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'schedule workflow'
`

		opts1 := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, fmt.Sprintf("create %s", wf1TreePath), wf1FileContent)
		createWorkflowFile(t, token, user2.Name, repo.Name, wf1TreePath, opts1)

		// fetch the task triggered by push
		task1 := runner.fetchTask(t)
		_, _, run1 := getTaskAndJobAndRunByTaskID(t, task1.Id)
		assert.Equal(t, "schedule-concurrency", run1.ConcurrencyGroup)
		assert.True(t, run1.ConcurrencyCancel)
		assert.Equal(t, string(webhook_module.HookEventPush), run1.TriggerEvent)
		assert.Equal(t, actions_model.StatusRunning, run1.Status)

		// trigger the task by schedule
		spec := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionScheduleSpec{RepoID: repo.ID})
		spec.Next = timeutil.TimeStampNow() // manually update "Next"
		assert.NoError(t, actions_model.UpdateScheduleSpec(context.Background(), spec, "next"))
		assert.NoError(t, actions_service.StartScheduleTasks(context.Background()))
		runner.fetchNoTask(t) // cannot fetch because task1 is not completed
		runner.execTask(t, task1, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})
		_, _, run1 = getTaskAndJobAndRunByTaskID(t, task1.Id)
		assert.Equal(t, actions_model.StatusSuccess, run1.Status)
		task2 := runner.fetchTask(t)
		_, _, run2 := getTaskAndJobAndRunByTaskID(t, task2.Id)
		assert.Equal(t, "schedule-concurrency", run2.ConcurrencyGroup)
		assert.False(t, run2.ConcurrencyCancel)
		assert.Equal(t, string(webhook_module.HookEventSchedule), run2.TriggerEvent)
		assert.Equal(t, actions_model.StatusRunning, run2.Status)

		// trigger the task by schedule again
		spec = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionScheduleSpec{RepoID: repo.ID})
		spec.Next = timeutil.TimeStampNow() // manually update "Next"
		assert.NoError(t, actions_model.UpdateScheduleSpec(context.Background(), spec, "next"))
		assert.NoError(t, actions_service.StartScheduleTasks(context.Background()))
		runner.fetchNoTask(t) // cannot fetch because task2 is not completed
		run3 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: repo.ID, Status: actions_model.StatusBlocked})
		assert.Equal(t, "schedule-concurrency", run3.ConcurrencyGroup)
		assert.False(t, run3.ConcurrencyCancel)
		assert.Equal(t, string(webhook_module.HookEventSchedule), run3.TriggerEvent)

		// trigger the task by push
		doAPICreateFile(httpContext, "doc.txt", &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				NewBranchName: "main",
				Message:       "create doc.txt",
				Author: api.Identity{
					Name:  user2.Name,
					Email: user2.Email,
				},
				Committer: api.Identity{
					Name:  user2.Name,
					Email: user2.Email,
				},
				Dates: api.CommitDateOptions{
					Author:    time.Now(),
					Committer: time.Now(),
				},
			},
			ContentBase64: base64.StdEncoding.EncodeToString([]byte("doc")),
		})(t)

		task4 := runner.fetchTask(t)
		_, _, run4 := getTaskAndJobAndRunByTaskID(t, task4.Id)
		assert.Equal(t, "schedule-concurrency", run4.ConcurrencyGroup)
		assert.True(t, run4.ConcurrencyCancel)
		assert.Equal(t, string(webhook_module.HookEventPush), run4.TriggerEvent)
		run3 = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run3.ID})
		assert.Equal(t, actions_model.StatusCancelled, run3.Status)
	})
}

func TestWorkflowAndJobConcurrency(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-concurrency", false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})

		httpContext := NewAPITestContext(t, user2.Name, repo.Name, auth_model.AccessTokenScopeWriteRepository)
		defer doAPIDeleteRepository(httpContext)(t)

		runner1 := newMockRunner()
		runner1.registerAsRepoRunner(t, user2.Name, repo.Name, "mock-runner-1", []string{"runner1"})
		runner2 := newMockRunner()
		runner2.registerAsRepoRunner(t, user2.Name, repo.Name, "mock-runner-2", []string{"runner2"})

		wf1TreePath := ".gitea/workflows/concurrent-workflow-1.yml"
		wf1FileContent := `name: concurrent-workflow-1
on: 
  push:
    paths:
      - '.gitea/workflows/concurrent-workflow-1.yml'
concurrency:
  group: workflow-group-1
jobs:
  wf1-job1:
    runs-on: runner1
    concurrency:
      group: job-group-1
    steps:
      - run: echo 'wf1-job1'
  wf1-job2:
    runs-on: runner2
    concurrency:
      group: job-group-2
    steps:
      - run: echo 'wf1-job2'
`
		wf2TreePath := ".gitea/workflows/concurrent-workflow-2.yml"
		wf2FileContent := `name: concurrent-workflow-2
on: 
  push:
    paths:
      - '.gitea/workflows/concurrent-workflow-2.yml'
concurrency:
  group: workflow-group-1
jobs:
  wf2-job1:
    runs-on: runner1
    concurrency:
      group: job-group-1
    steps:
      - run: echo 'wf2-job2'
  wf2-job2:
    runs-on: runner2
    concurrency:
      group: job-group-2
    steps:
      - run: echo 'wf2-job2'
`
		wf3TreePath := ".gitea/workflows/concurrent-workflow-3.yml"
		wf3FileContent := `name: concurrent-workflow-3
on: 
  push:
    paths:
      - '.gitea/workflows/concurrent-workflow-3.yml'
concurrency:
  group: workflow-group-2
jobs:
  wf3-job1:
    runs-on: runner1
    concurrency:
      group: job-group-1
    steps:
      - run: echo 'wf3-job1'
`

		wf4TreePath := ".gitea/workflows/concurrent-workflow-4.yml"
		wf4FileContent := `name: concurrent-workflow-4
on: 
  push:
    paths:
      - '.gitea/workflows/concurrent-workflow-4.yml'
concurrency:
  group: workflow-group-2
jobs:
  wf4-job1:
    runs-on: runner2
    concurrency:
      group: job-group-2
      cancel-in-progress: true
    steps:
      - run: echo 'wf4-job1'
`

		// push workflow 1, 2 and 3
		opts1 := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, fmt.Sprintf("create %s", wf1TreePath), wf1FileContent)
		createWorkflowFile(t, token, user2.Name, repo.Name, wf1TreePath, opts1)
		opts2 := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, fmt.Sprintf("create %s", wf2TreePath), wf2FileContent)
		createWorkflowFile(t, token, user2.Name, repo.Name, wf2TreePath, opts2)
		opts3 := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, fmt.Sprintf("create %s", wf3TreePath), wf3FileContent)
		createWorkflowFile(t, token, user2.Name, repo.Name, wf3TreePath, opts3)
		// fetch wf1-job1 and wf1-job2
		w1j1Task := runner1.fetchTask(t)
		w1j2Task := runner2.fetchTask(t)
		// cannot fetch wf2-job1 and wf2-job2 because workflow-2 is blocked by workflow-1's concurrency group "workflow-group-1"
		// cannot fetch wf3-job1 because it is blocked by wf1-job1's concurrency group "job-group-1"
		runner1.fetchNoTask(t)
		runner2.fetchNoTask(t)
		_, w1j1Job, w1Run := getTaskAndJobAndRunByTaskID(t, w1j1Task.Id)
		assert.Equal(t, "job-group-1", w1j1Job.ConcurrencyGroup)
		assert.Equal(t, "workflow-group-1", w1Run.ConcurrencyGroup)
		assert.Equal(t, "concurrent-workflow-1.yml", w1Run.WorkflowID)
		_, w1j2Job, _ := getTaskAndJobAndRunByTaskID(t, w1j2Task.Id)
		assert.Equal(t, "job-group-2", w1j2Job.ConcurrencyGroup)
		// exec wf1-job1
		runner1.execTask(t, w1j1Task, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})
		// fetch wf3-job1
		w3j1Task := runner1.fetchTask(t)
		// cannot fetch wf2-job1 and wf2-job2 because workflow-2 is blocked by workflow-1's concurrency group "workflow-group-1"
		runner1.fetchNoTask(t)
		runner2.fetchNoTask(t)
		_, w3j1Job, w3Run := getTaskAndJobAndRunByTaskID(t, w3j1Task.Id)
		assert.Equal(t, "job-group-1", w3j1Job.ConcurrencyGroup)
		assert.Equal(t, "workflow-group-2", w3Run.ConcurrencyGroup)
		assert.Equal(t, "concurrent-workflow-3.yml", w3Run.WorkflowID)
		// push workflow-4
		opts4 := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, fmt.Sprintf("create %s", wf4TreePath), wf4FileContent)
		createWorkflowFile(t, token, user2.Name, repo.Name, wf4TreePath, opts4)
		// exec wf1-job2
		runner2.execTask(t, w1j2Task, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})
		// wf2-job2
		w2j2Task := runner2.fetchTask(t)
		// cannot fetch wf2-job1 because it is blocked by wf3-job1's concurrency group "job-group-1"
		// cannot fetch wf4-job1 because it is blocked by workflow-3's concurrency group "workflow-group-2"
		runner1.fetchNoTask(t)
		runner2.fetchNoTask(t)
		_, w2j2Job, w2Run := getTaskAndJobAndRunByTaskID(t, w2j2Task.Id)
		assert.Equal(t, "job-group-2", w2j2Job.ConcurrencyGroup)
		assert.Equal(t, "workflow-group-1", w2Run.ConcurrencyGroup)
		assert.Equal(t, "concurrent-workflow-2.yml", w2Run.WorkflowID)
		// exec wf3-job1
		runner1.execTask(t, w3j1Task, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})
		// fetch wf2-job1
		w2j1Task := runner1.fetchTask(t)
		// fetch wf4-job1
		w4j1Task := runner2.fetchTask(t)
		// all tasks have been fetched
		runner1.fetchNoTask(t)
		runner2.fetchNoTask(t)
		_, w2j1Job, _ := getTaskAndJobAndRunByTaskID(t, w2j1Task.Id)
		assert.Equal(t, "job-group-1", w2j1Job.ConcurrencyGroup)
		assert.Equal(t, actions_model.StatusRunning, w2j2Job.Status)
		_, w2j2Job, w2Run = getTaskAndJobAndRunByTaskID(t, w2j2Task.Id)
		// wf2-job2 is cancelled because wf4-job1's cancel-in-progress is true
		assert.Equal(t, actions_model.StatusCancelled, w2j2Job.Status)
		assert.Equal(t, actions_model.StatusCancelled, w2Run.Status)
		_, w4j1Job, w4Run := getTaskAndJobAndRunByTaskID(t, w4j1Task.Id)
		assert.Equal(t, "job-group-2", w4j1Job.ConcurrencyGroup)
		assert.Equal(t, "workflow-group-2", w4Run.ConcurrencyGroup)
		assert.Equal(t, "concurrent-workflow-4.yml", w4Run.WorkflowID)
	})
}

func getTaskAndJobAndRunByTaskID(t *testing.T, taskID int64) (*actions_model.ActionTask, *actions_model.ActionRunJob, *actions_model.ActionRun) {
	actionTask := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: taskID})
	actionRunJob := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: actionTask.JobID})
	actionRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: actionRunJob.RunID})
	return actionTask, actionRunJob, actionRun
}
