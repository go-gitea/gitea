// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	runnerv1 "gitea.dev/actions-proto-go/runner/v1"
	actions_model "gitea.dev/models/actions"
	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/gitrepo"
	"gitea.dev/modules/json"
	api "gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsReusableWorkflow(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		user2Session := loginUser(t, user2.Name)
		user2Token := getTokenForLoggedInUser(t, user2Session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
		user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
		user4Session := loginUser(t, user4.Name)
		user4Token := getTokenForLoggedInUser(t, user4Session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		t.Run("Same-repo reusable workflow", func(t *testing.T) {
			apiRepo := createActionsTestRepo(t, user2Token, "workflow-call-test", false)
			repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})

			defaultRunner := newMockRunner()
			defaultRunner.registerAsRepoRunner(t, repo.OwnerName, repo.Name, "mock-default-runner", []string{"ubuntu-latest"}, false)
			customRunner := newMockRunner()
			customRunner.registerAsRepoRunner(t, repo.OwnerName, repo.Name, "mock-custom-runner", []string{"custom-os"}, false)

			// add a variable for test
			req := NewRequestWithJSON(t, "POST",
				fmt.Sprintf("/api/v1/repos/%s/%s/actions/variables/myvar", repo.OwnerName, repo.Name), &api.CreateVariableOption{
					Value: "abcdef",
				}).
				AddTokenAuth(user2Token)
			MakeRequest(t, req, http.StatusCreated)
			// add a secret for test
			req = NewRequestWithJSON(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/%s/actions/secrets/mysecret", repo.OwnerName, repo.Name), api.CreateOrUpdateSecretOption{
				Data: "secRET-t0Ken",
			}).AddTokenAuth(user2Token)
			MakeRequest(t, req, http.StatusCreated)

			createRepoWorkflowFile(t, user2, user2Token, repo, ".gitea/workflows/reusable1.yaml",
				`name: Reusable1
on:
  workflow_call:
    inputs:
      str_input:
        type: string
      num_input:
       type: number
      bool_input:
       type: boolean
      parent_var:
        type: string
      needs_out:
        type: string
    secrets:
      PARENT_TOKEN:
    outputs:
      r1_out:
        value: ${{ jobs.reusable1_job2.outputs.r1j2_out }}

jobs:
  reusable1_job1:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'reusable1_job1'

  reusable1_job2:
    needs: [reusable1_job1]
    outputs:
      r1j2_out: ${{ steps.gen_r1j2_output.outputs.out }}
    runs-on: custom-os
    steps:
      - id: gen_r1j2_output
        run: |
          echo "out=r1j2_out_data" >> "$GITHUB_OUTPUT"

  reusable1_job3:
    needs: [reusable1_job2]
    uses: ./.gitea/workflows/reusable2.yaml
    with:
      msg: ${{ inputs.str_input }}
`)

			createRepoWorkflowFile(t, user2, user2Token, repo, ".gitea/workflows/reusable2.yaml",
				`name: Reusable2
on:
  workflow_call:
    inputs:
      msg:
        type: string

jobs:
  reusable2_job1:
    runs-on: ubuntu-latest
    steps:
      - run: echo ${{ inputs.msg }}
`)

			createRepoWorkflowFile(t, user2, user2Token, repo, ".gitea/workflows/caller.yaml",
				`name: Caller
on:
  push:
    paths:
      - '.gitea/workflows/caller.yaml'
jobs:
  caller_job1:
    runs-on: ubuntu-latest
    outputs:
      prepared: ${{ steps.gen_output.outputs.pd }}
    steps:
      - id: gen_output
        run: |
          echo "pd=prepared_data" >> "$GITHUB_OUTPUT"

  caller_job2:
    needs: [caller_job1]
    uses: './.gitea/workflows/reusable1.yaml'
    with:
      str_input: 'from_caller_job2'
      num_input: ${{ 2.3e2 }}
      bool_input: ${{ gitea.event_name == 'push' }}
      parent_var: ${{ vars.myvar }}
      needs_out: ${{ needs.caller_job1.outputs.prepared }}
    secrets:
      PARENT_TOKEN: ${{ secrets.mysecret }}

  caller_job3:
    needs: [caller_job2]
    runs-on: ubuntu-latest
    steps:
      - run: |
          echo ${{ needs.caller_job1.outputs.r1_out }}
`)

			var (
				runID                                int64
				callerJob2ID, callerJob2AttemptJobID int64
				callerJob3AttemptJobID               int64
				r1Job2ID, r1Job2AttemptJobID         int64
				r1Job3ID, r1Job3AttemptJobID         int64
				r2Job1AttemptJobID                   int64
			)

			t.Run("Check initialized jobs", func(t *testing.T) {
				// run
				assert.Equal(t, 1, unittest.GetCount(t, &actions_model.ActionRun{RepoID: repo.ID}))
				run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: repo.ID})
				runID = run.ID

				// caller_job1
				assert.Equal(t, 3, unittest.GetCount(t, &actions_model.ActionRunJob{RunID: runID}))
				callerJob1 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RunID: runID, JobID: "caller_job1"})
				assert.Equal(t, actions_model.StatusWaiting, callerJob1.Status)
				assert.False(t, callerJob1.IsReusableCaller)

				// caller_job2
				callerJob2 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RunID: runID, JobID: "caller_job2"})
				callerJob2ID = callerJob2.ID
				callerJob2AttemptJobID = callerJob2.AttemptJobID
				assert.Equal(t, actions_model.StatusBlocked, callerJob2.Status)
				assert.True(t, callerJob2.IsReusableCaller)

				// caller_job3
				callerJob3 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RunID: runID, JobID: "caller_job3"})
				callerJob3AttemptJobID = callerJob3.AttemptJobID
				assert.Equal(t, actions_model.StatusBlocked, callerJob3.Status)
				assert.False(t, callerJob3.IsReusableCaller)
			})

			t.Run("First run", func(t *testing.T) {
				callerJob1Task := defaultRunner.fetchTask(t) // for caller_job1
				_, callerJob1, _ := getTaskAndJobAndRunByTaskID(t, callerJob1Task.Id)
				assert.Equal(t, "caller_job1", callerJob1.JobID)
				defaultRunner.fetchNoTask(t)
				defaultRunner.execTask(t, callerJob1Task, &mockTaskOutcome{
					result: runnerv1.Result_RESULT_SUCCESS,
					outputs: map[string]string{
						"prepared": "prepared_data",
					},
				})

				r1Job1Task := defaultRunner.fetchTask(t) // for reusable1_job1
				_, r1Job1, _ := getTaskAndJobAndRunByTaskID(t, r1Job1Task.Id)
				assert.Equal(t, "reusable1_job1", r1Job1.JobID)
				assert.Equal(t, callerJob2ID, r1Job1.ParentJobID)
				payload := getWorkflowCallPayloadFromTask(t, r1Job1Task)
				if assert.Len(t, payload.Inputs, 5) {
					assert.Equal(t, "from_caller_job2", payload.Inputs["str_input"])
					assert.EqualValues(t, 230, payload.Inputs["num_input"])
					assert.Equal(t, true, payload.Inputs["bool_input"])
					assert.Equal(t, "abcdef", payload.Inputs["parent_var"])
					assert.Equal(t, "prepared_data", payload.Inputs["needs_out"])
				}
				if assert.Len(t, r1Job1Task.Secrets, 3) {
					assert.Contains(t, r1Job1Task.Secrets, "GITEA_TOKEN")
					assert.Contains(t, r1Job1Task.Secrets, "GITHUB_TOKEN")
					assert.Equal(t, "secRET-t0Ken", r1Job1Task.Secrets["PARENT_TOKEN"])
				}
				customRunner.fetchNoTask(t)
				defaultRunner.execTask(t, r1Job1Task, &mockTaskOutcome{
					result: runnerv1.Result_RESULT_SUCCESS,
				})

				// reusable1_job3 (a nested caller) needs reusable1_job2, so it stays Blocked until r1j2 succeeds.
				r1Job3Pre := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RunID: runID, JobID: "reusable1_job3"})
				assert.Equal(t, actions_model.StatusBlocked, r1Job3Pre.Status)
				assert.False(t, r1Job3Pre.IsExpanded)
				assert.Equal(t, 0, unittest.GetCount(t, &actions_model.ActionRunJob{RunID: runID, JobID: "reusable2_job1"}))

				r1Job2Task := customRunner.fetchTask(t) // for reusable1_job2
				_, r1Job2, _ := getTaskAndJobAndRunByTaskID(t, r1Job2Task.Id)
				assert.Equal(t, "reusable1_job2", r1Job2.JobID)
				r1Job2ID = r1Job2.ID
				r1Job2AttemptJobID = r1Job2.AttemptJobID
				if assert.Len(t, r1Job2Task.Needs, 1) {
					assert.Contains(t, r1Job2Task.Needs, "reusable1_job1")
					assert.Equal(t, runnerv1.Result_RESULT_SUCCESS, r1Job2Task.Needs["reusable1_job1"].Result)
				}
				customRunner.execTask(t, r1Job2Task, &mockTaskOutcome{
					result: runnerv1.Result_RESULT_SUCCESS,
					outputs: map[string]string{
						"r1j2_out": "r1j2_out_data",
					},
				})

				// Now reusable1_job3 expands and reusable2_job1 becomes runnable.
				r1Job3 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RunID: runID, JobID: "reusable1_job3"})
				assert.True(t, r1Job3.IsReusableCaller)
				assert.True(t, r1Job3.IsExpanded)
				assert.Equal(t, callerJob2ID, r1Job3.ParentJobID)
				r1Job3ID = r1Job3.ID
				r1Job3AttemptJobID = r1Job3.AttemptJobID
				r2Job1 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RunID: runID, JobID: "reusable2_job1"})
				assert.Equal(t, r1Job3ID, r2Job1.ParentJobID)
				r2Job1AttemptJobID = r2Job1.AttemptJobID

				r2Job1Task := defaultRunner.fetchTask(t) // for reusable2_job1
				_, fetchedR2Job1, _ := getTaskAndJobAndRunByTaskID(t, r2Job1Task.Id)
				assert.Equal(t, "reusable2_job1", fetchedR2Job1.JobID)
				assert.Equal(t, r1Job3ID, fetchedR2Job1.ParentJobID)
				r2Job1Payload := getWorkflowCallPayloadFromTask(t, r2Job1Task)
				if assert.Len(t, r2Job1Payload.Inputs, 1) {
					assert.Equal(t, "from_caller_job2", r2Job1Payload.Inputs["msg"])
				}
				defaultRunner.execTask(t, r2Job1Task, &mockTaskOutcome{
					result: runnerv1.Result_RESULT_SUCCESS,
				})

				callerJob2 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: callerJob2ID})
				assert.Equal(t, actions_model.StatusSuccess, callerJob2.Status)

				callerJob3Task := defaultRunner.fetchTask(t) // for caller_job3
				_, callerJob3, _ := getTaskAndJobAndRunByTaskID(t, callerJob3Task.Id)
				assert.Equal(t, "caller_job3", callerJob3.JobID)
				if assert.Len(t, callerJob3Task.Needs, 1) {
					assert.Contains(t, callerJob3Task.Needs, "caller_job2")
					assert.Equal(t, runnerv1.Result_RESULT_SUCCESS, callerJob3Task.Needs["caller_job2"].Result)
					if assert.Len(t, callerJob3Task.Needs["caller_job2"].Outputs, 1) {
						assert.Equal(t, "r1j2_out_data", callerJob3Task.Needs["caller_job2"].Outputs["r1_out"])
					}
				}
				defaultRunner.execTask(t, callerJob3Task, &mockTaskOutcome{
					result: runnerv1.Result_RESULT_SUCCESS,
				})
				callerRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runID})
				assert.Equal(t, actions_model.StatusSuccess, callerRun.Status)
			})

			t.Run("Rerun 'reusable1_job2'", func(t *testing.T) {
				req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d/rerun", repo.OwnerName, repo.Name, runID, r1Job2ID))
				user2Session.MakeRequest(t, req, http.StatusOK)

				run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runID})
				assert.Equal(t, actions_model.StatusWaiting, run.Status)
				attempt2 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunAttempt{RunID: runID, Attempt: 2})
				assert.Equal(t, actions_model.StatusWaiting, attempt2.Status)
				callerJob2 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RunID: runID, RunAttemptID: attempt2.ID, AttemptJobID: callerJob2AttemptJobID})
				assert.Equal(t, actions_model.StatusWaiting, callerJob2.Status)
				callerJob3 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RunID: runID, RunAttemptID: attempt2.ID, AttemptJobID: callerJob3AttemptJobID})
				assert.Equal(t, actions_model.StatusBlocked, callerJob3.Status)

				// reusable1_job3 needs reusable1_job2, so rerunning r1j2 pulls r1j3 (and its subtree) into the rerun set
				r1Job3Attempt2 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RunID: runID, RunAttemptID: attempt2.ID, AttemptJobID: r1Job3AttemptJobID})
				assert.Equal(t, actions_model.StatusBlocked, r1Job3Attempt2.Status)
				assert.True(t, r1Job3Attempt2.IsReusableCaller)
				assert.False(t, r1Job3Attempt2.IsExpanded)
				assert.Equal(t, 0, unittest.GetCount(t, &actions_model.ActionRunJob{RunID: runID, RunAttemptID: attempt2.ID, JobID: "reusable2_job1"}))

				defaultRunner.fetchNoTask(t)
				r1Job2Task := customRunner.fetchTask(t)
				_, r1Job2, _ := getTaskAndJobAndRunByTaskID(t, r1Job2Task.Id)
				assert.Equal(t, "reusable1_job2", r1Job2.JobID)
				assert.Equal(t, callerJob2.ID, r1Job2.ParentJobID)
				assert.Equal(t, r1Job2AttemptJobID, r1Job2.AttemptJobID)
				assert.Equal(t, actions_model.StatusRunning, r1Job2.Status)
				run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runID})
				assert.Equal(t, actions_model.StatusRunning, run.Status)
				customRunner.execTask(t, r1Job2Task, &mockTaskOutcome{
					result: runnerv1.Result_RESULT_SUCCESS,
					outputs: map[string]string{
						"r1j2_out": "r1j2_out_data_updated",
					},
				})

				// r1j3 expands again. Its child reuses the AttemptJobID from attempt 1
				r1Job3Attempt2 = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RunID: runID, RunAttemptID: attempt2.ID, AttemptJobID: r1Job3AttemptJobID})
				assert.True(t, r1Job3Attempt2.IsExpanded)
				r2Job1Attempt2 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RunID: runID, RunAttemptID: attempt2.ID, JobID: "reusable2_job1"})
				assert.Equal(t, r2Job1AttemptJobID, r2Job1Attempt2.AttemptJobID)
				assert.Equal(t, r1Job3Attempt2.ID, r2Job1Attempt2.ParentJobID)

				r2Job1Task := defaultRunner.fetchTask(t)
				_, fetchedR2Job1, _ := getTaskAndJobAndRunByTaskID(t, r2Job1Task.Id)
				assert.Equal(t, "reusable2_job1", fetchedR2Job1.JobID)
				defaultRunner.execTask(t, r2Job1Task, &mockTaskOutcome{
					result: runnerv1.Result_RESULT_SUCCESS,
				})

				callerJob2 = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: callerJob2.ID})
				assert.Equal(t, actions_model.StatusSuccess, callerJob2.Status)

				callerJob3Task := defaultRunner.fetchTask(t)
				_, callerJob3, _ = getTaskAndJobAndRunByTaskID(t, callerJob3Task.Id)
				assert.Equal(t, "caller_job3", callerJob3.JobID)
				if assert.Len(t, callerJob3Task.Needs, 1) {
					assert.Contains(t, callerJob3Task.Needs, "caller_job2")
					assert.Equal(t, runnerv1.Result_RESULT_SUCCESS, callerJob3Task.Needs["caller_job2"].Result)
					if assert.Len(t, callerJob3Task.Needs["caller_job2"].Outputs, 1) {
						assert.Equal(t, "r1j2_out_data_updated", callerJob3Task.Needs["caller_job2"].Outputs["r1_out"])
					}
				}
				defaultRunner.execTask(t, callerJob3Task, &mockTaskOutcome{
					result: runnerv1.Result_RESULT_SUCCESS,
				})
				attempt2 = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunAttempt{RunID: runID, Attempt: 2})
				assert.Equal(t, actions_model.StatusSuccess, attempt2.Status)
				run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runID})
				assert.Equal(t, actions_model.StatusSuccess, run.Status)
			})
		})

		t.Run("Cross-repo reusable workflow with collaborative owner", func(t *testing.T) {
			// libRepo: private, owned by user2.
			libAPIRepo := createActionsTestRepo(t, user2Token, "reusable-lib-private", true)
			libRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: libAPIRepo.ID})
			createRepoWorkflowFile(t, user2, user2Token, libRepo, ".gitea/workflows/reusable_lib.yaml",
				`name: ReusableLib
on:
  workflow_call:
    inputs:
      from:
        type: string

jobs:
  lib_job:
    runs-on: ubuntu-latest
    steps:
      - run: echo hello-${{ inputs.from }}
`)

			// consumerRepo: private, owned by user4.
			consumerAPIRepo := createActionsTestRepo(t, user4Token, "workflow-call-cross-repo", true)
			consumerRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: consumerAPIRepo.ID})

			runner := newMockRunner()
			runner.registerAsRepoRunner(t, consumerRepo.OwnerName, consumerRepo.Name, "mock-cross-runner", []string{"ubuntu-latest"}, false)

			createRepoWorkflowFile(t, user4, user4Token, consumerRepo, ".gitea/workflows/cross-caller.yaml",
				`name: CrossCaller
on: push
jobs:
  cross_job:
    uses: user2/reusable-lib-private/.gitea/workflows/reusable_lib.yaml@main
    with:
      from: 'consumer'
`)

			// Phase 1: no grant. The cross-repo read check fails, and NO ActionRun row gets persisted.
			assert.Equal(t, 0, unittest.GetCount(t, &actions_model.ActionRun{RepoID: consumerRepo.ID}))
			runner.fetchNoTask(t)

			// Phase 2: user2 (libRepo owner) adds user4 (consumer owner) as a Collaborative Owner of libRepo.
			addCollabReq := NewRequestWithValues(t, "POST",
				fmt.Sprintf("/%s/%s/settings/actions/general/collaborative_owner/add", libRepo.OwnerName, libRepo.Name),
				map[string]string{"collaborative_owner": user4.Name})
			user2Session.MakeRequest(t, addCollabReq, http.StatusOK)

			// Phase 3: trigger the workflow again
			createRepoWorkflowFile(t, user4, user4Token, consumerRepo, "marker.txt", "trigger after grant")

			run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: consumerRepo.ID})
			crossJob := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RunID: run.ID, JobID: "cross_job"})
			assert.True(t, crossJob.IsReusableCaller)
			assert.True(t, crossJob.IsExpanded)
			assert.Equal(t, actions_model.StatusWaiting, crossJob.Status)

			libJobTask := runner.fetchTask(t)
			_, fetchedLibJob, _ := getTaskAndJobAndRunByTaskID(t, libJobTask.Id)
			assert.Equal(t, "lib_job", fetchedLibJob.JobID)
			assert.Equal(t, crossJob.ID, fetchedLibJob.ParentJobID)
			assert.Equal(t, consumerRepo.ID, fetchedLibJob.RepoID)
			payload := getWorkflowCallPayloadFromTask(t, libJobTask)
			if assert.Len(t, payload.Inputs, 1) {
				assert.Equal(t, "consumer", payload.Inputs["from"])
			}
			crossJob = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: crossJob.ID})
			assert.Equal(t, actions_model.StatusRunning, crossJob.Status)
			runner.execTask(t, libJobTask, &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})

			crossJob = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: crossJob.ID})
			assert.Equal(t, actions_model.StatusSuccess, crossJob.Status)
			run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
			assert.Equal(t, actions_model.StatusSuccess, run.Status)
		})

		t.Run("Public caller denied private target even with collaborative owner", func(t *testing.T) {
			// Isolates the run.Repo.IsPrivate gate: a public caller must be denied a private target even with a
			// collaborative-owner grant, since allowing it would expose private workflow content in a public run.

			// libRepo: private, owned by user2.
			libAPIRepo := createActionsTestRepo(t, user2Token, "reusable-lib-public-denied", true)
			libRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: libAPIRepo.ID})
			createRepoWorkflowFile(t, user2, user2Token, libRepo, ".gitea/workflows/reusable_lib.yaml",
				`name: ReusableLib
on:
  workflow_call:

jobs:
  lib_job:
    runs-on: ubuntu-latest
    steps:
      - run: echo hello
`)

			// Grant first: user2 adds user4 as a collaborative owner of the private libRepo, so the grant is
			// satisfied and the public-caller gate is the only thing that can deny access.
			addCollabReq := NewRequestWithValues(t, "POST",
				fmt.Sprintf("/%s/%s/settings/actions/general/collaborative_owner/add", libRepo.OwnerName, libRepo.Name),
				map[string]string{"collaborative_owner": user4.Name})
			user2Session.MakeRequest(t, addCollabReq, http.StatusOK)

			// consumerRepo: public, owned by user4.
			consumerAPIRepo := createActionsTestRepo(t, user4Token, "workflow-call-public-denied", false)
			consumerRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: consumerAPIRepo.ID})

			runner := newMockRunner()
			runner.registerAsRepoRunner(t, consumerRepo.OwnerName, consumerRepo.Name, "mock-public-denied-runner", []string{"ubuntu-latest"}, false)

			createRepoWorkflowFile(t, user4, user4Token, consumerRepo, ".gitea/workflows/cross-caller.yaml",
				`name: CrossCaller
on: push
jobs:
  cross_job:
    uses: user2/reusable-lib-public-denied/.gitea/workflows/reusable_lib.yaml@main
`)

			// Denied: the cross-repo read check fails for the public caller, so NO ActionRun is persisted and no task is dispatched.
			assert.Equal(t, 0, unittest.GetCount(t, &actions_model.ActionRun{RepoID: consumerRepo.ID}))
			runner.fetchNoTask(t)
		})

		t.Run("Cross-repo callee with same-repo nested uses", func(t *testing.T) {
			// A same-repo `uses: ./...` inside a cross-repo reusable callee must resolve relative to the callee's own repo (matching GitHub's behavior), not the original triggering repo.

			// Place a util.yaml with a distinguishable job name in BOTH repos to detect mis-resolution.

			libAPIRepo := createActionsTestRepo(t, user2Token, "reusable-lib-nested", false)
			libRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: libAPIRepo.ID})
			createRepoWorkflowFile(t, user2, user2Token, libRepo, ".gitea/workflows/util.yaml",
				`name: UtilLib
on:
  workflow_call:

jobs:
  util_lib_job:
    runs-on: ubuntu-latest
    steps:
      - run: echo from-lib
`)
			createRepoWorkflowFile(t, user2, user2Token, libRepo, ".gitea/workflows/lib.yaml",
				`name: LibNested
on:
  workflow_call:

jobs:
  call_util_in_lib:
    uses: ./.gitea/workflows/util.yaml
`)

			consumerAPIRepo := createActionsTestRepo(t, user4Token, "consumer-nested-uses", false)
			consumerRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: consumerAPIRepo.ID})

			// A *different* util.yaml in the consumer repo: if `./` mis-resolves we'd see this job's name.
			createRepoWorkflowFile(t, user4, user4Token, consumerRepo, ".gitea/workflows/util.yaml",
				`name: UtilConsumer
on:
  workflow_call:

jobs:
  util_consumer_job:
    runs-on: ubuntu-latest
    steps:
      - run: echo from-consumer
`)

			runner := newMockRunner()
			runner.registerAsRepoRunner(t, consumerRepo.OwnerName, consumerRepo.Name, "mock-nested-runner", []string{"ubuntu-latest"}, false)

			createRepoWorkflowFile(t, user4, user4Token, consumerRepo, ".gitea/workflows/caller.yaml",
				`name: NestedCaller
on: push
jobs:
  cross_job:
    uses: user2/reusable-lib-nested/.gitea/workflows/lib.yaml@main
`)

			run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: consumerRepo.ID})
			crossJob := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RunID: run.ID, JobID: "cross_job"})
			assert.True(t, crossJob.IsReusableCaller)
			assert.True(t, crossJob.IsExpanded)

			// cross_job's children come from libRepo/lib.yaml - their source must be libRepo + libRepo's commit.
			libHead, err := gitrepo.GetBranchCommitID(t.Context(), libRepo, libRepo.DefaultBranch)
			require.NoError(t, err)
			callUtilJob := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RunID: run.ID, JobID: "call_util_in_lib", ParentJobID: crossJob.ID})
			assert.True(t, callUtilJob.IsReusableCaller)
			assert.Equal(t, libRepo.ID, callUtilJob.WorkflowSourceRepoID)
			assert.Equal(t, libHead, callUtilJob.WorkflowSourceCommitSHA)

			// call_util_in_lib has `uses: ./.gitea/workflows/util.yaml`, so its children should come from libRepo/util.yaml
			assert.True(t, callUtilJob.IsExpanded)
			unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RunID: run.ID, JobID: "util_lib_job", ParentJobID: callUtilJob.ID})
			unittest.AssertNotExistsBean(t, &actions_model.ActionRunJob{RunID: run.ID, JobID: "util_consumer_job"})
		})

		t.Run("Missing callee file", func(t *testing.T) {
			// A caller workflow references a callee path that does not exist in the repo.

			apiRepo := createActionsTestRepo(t, user2Token, "caller-missing-callee", false)
			repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})

			createRepoWorkflowFile(t, user2, user2Token, repo, ".gitea/workflows/caller.yaml",
				`name: Caller
on: push
jobs:
  plain_job:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'job'
  call_missing:
    uses: ./.gitea/workflows/does-not-exist.yml
`)

			assert.Equal(t, 0, unittest.GetCount(t, &actions_model.ActionRun{RepoID: repo.ID}))
		})

		t.Run("Fork PR with secrets: inherit does not leak base repo secrets", func(t *testing.T) {
			// user2 owns the base repo, configures a secret, and registers a reusable workflow that declares a required secret.
			// The caller workflow uses `secrets: inherit`.

			apiBaseRepo := createActionsTestRepo(t, user2Token, "fork-pr-inherit-test", false)
			baseRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiBaseRepo.ID})
			user2APICtx := NewAPITestContext(t, baseRepo.OwnerName, baseRepo.Name, auth_model.AccessTokenScopeWriteRepository)
			defer doAPIDeleteRepository(user2APICtx)(t)

			// Real secret that must never reach a fork PR task.
			req := NewRequestWithJSON(t, "PUT",
				fmt.Sprintf("/api/v1/repos/%s/%s/actions/secrets/leaked_secret", baseRepo.OwnerName, baseRepo.Name),
				api.CreateOrUpdateSecretOption{Data: "MUST-NOT-LEAK"}).AddTokenAuth(user2Token)
			MakeRequest(t, req, http.StatusCreated)

			runner := newMockRunner()
			runner.registerAsRepoRunner(t, baseRepo.OwnerName, baseRepo.Name, "mock-fork-runner", []string{"ubuntu-latest"}, false)

			createRepoWorkflowFile(t, user2, user2Token, baseRepo, ".gitea/workflows/reusable.yaml",
				`name: Reusable
on:
  workflow_call:
    secrets:
      leaked_secret:

jobs:
  callee:
    runs-on: ubuntu-latest
    steps:
      - run: echo
`)
			createRepoWorkflowFile(t, user2, user2Token, baseRepo, ".gitea/workflows/caller.yaml",
				`name: Caller
on: pull_request
jobs:
  call_reusable:
    uses: ./.gitea/workflows/reusable.yaml
    secrets: inherit
`)

			// user4 forks
			req = NewRequestWithJSON(t, "POST",
				fmt.Sprintf("/api/v1/repos/%s/%s/forks", baseRepo.OwnerName, baseRepo.Name),
				&api.CreateForkOption{Name: new("fork-pr-inherit-test-fork")}).AddTokenAuth(user4Token)
			resp := MakeRequest(t, req, http.StatusAccepted)
			apiForkRepo := DecodeJSON(t, resp, &api.Repository{})
			forkRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiForkRepo.ID})
			user4APICtx := NewAPITestContext(t, user4.Name, forkRepo.Name, auth_model.AccessTokenScopeWriteRepository)
			defer doAPIDeleteRepository(user4APICtx)(t)

			// user4 pushes a change on the fork and opens a PR to base
			doAPICreateFile(user4APICtx, "user4-fix.txt", &api.CreateFileOptions{
				FileOptions: api.FileOptions{
					NewBranchName: "user4/branch",
					Message:       "create user4-fix.txt",
					Author:        api.Identity{Name: user4.Name, Email: user4.Email},
					Committer:     api.Identity{Name: user4.Name, Email: user4.Email},
					Dates:         api.CommitDateOptions{Author: time.Now(), Committer: time.Now()},
				},
				ContentBase64: base64.StdEncoding.EncodeToString([]byte("fix")),
			})(t)
			doAPICreatePullRequest(user4APICtx, baseRepo.OwnerName, baseRepo.Name, baseRepo.DefaultBranch, user4.Name+":user4/branch")(t)

			// Approve the fork PR run.
			forkRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: baseRepo.ID, TriggerUserID: user4.ID})
			assert.True(t, forkRun.IsForkPullRequest)
			req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/approve", baseRepo.OwnerName, baseRepo.Name, forkRun.ID))
			user2Session.MakeRequest(t, req, http.StatusOK)

			task := runner.fetchTask(t)
			_, taskJob, taskRun := getTaskAndJobAndRunByTaskID(t, task.Id)
			assert.Equal(t, "callee", taskJob.JobID)
			assert.Equal(t, forkRun.ID, taskRun.ID)

			// Only the auto-issued tokens should be present. The user-defined `leaked_secret` must not appear.
			assert.Contains(t, task.Secrets, "GITEA_TOKEN")
			assert.Contains(t, task.Secrets, "GITHUB_TOKEN")
			assert.NotContains(t, task.Secrets, "leaked_secret")
			for name, value := range task.Secrets {
				assert.NotEqual(t, "MUST-NOT-LEAK", value, "secret %q leaked the base repo's secret value into a fork PR task", name)
			}

			runner.execTask(t, task, &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})
		})

		t.Run("Caller alternates expanding across attempts", func(t *testing.T) {
			apiRepo := createActionsTestRepo(t, user2Token, "caller-walkback-test", false)
			repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})

			runner := newMockRunner()
			runner.registerAsRepoRunner(t, repo.OwnerName, repo.Name, "mock-walkback-runner", []string{"ubuntu-latest"}, false)

			// Scenario:
			//   attempt 1: gate succeeds -> caller expands -> inner runs (records inner.AttemptJobID = N)
			//   attempt 2: rerun gate, mock Failure -> caller is Skipped without expanding (no children inserted)
			//   attempt 3: rerun gate, mock Success -> caller expands again -> inner.AttemptJobID must equal N

			createRepoWorkflowFile(t, user2, user2Token, repo, ".gitea/workflows/lib.yaml",
				`name: Lib
on:
  workflow_call:

jobs:
  inner:
    runs-on: ubuntu-latest
    steps:
      - run: echo inner
`)
			createRepoWorkflowFile(t, user2, user2Token, repo, ".gitea/workflows/main.yaml",
				`name: Main
on:
  push:
    paths:
      - '.gitea/workflows/main.yaml'
jobs:
  gate:
    runs-on: ubuntu-latest
    steps:
      - run: echo gate

  caller:
    needs: [gate]
    uses: ./.gitea/workflows/lib.yaml
`)

			run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: repo.ID})
			runID := run.ID

			latestAttempt := func() *actions_model.ActionRunAttempt {
				r := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runID})
				return unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunAttempt{ID: r.LatestAttemptID})
			}
			jobInLatest := func(jobID string) *actions_model.ActionRunJob {
				a := latestAttempt()
				return unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RunID: runID, RunAttemptID: a.ID, JobID: jobID})
			}

			// attempt 1: gate Success -> caller expands -> inner runs
			gate1Task := runner.fetchTask(t)
			_, gate1, _ := getTaskAndJobAndRunByTaskID(t, gate1Task.Id)
			assert.Equal(t, "gate", gate1.JobID)
			runner.execTask(t, gate1Task, &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})

			inner1Task := runner.fetchTask(t)
			_, inner1, _ := getTaskAndJobAndRunByTaskID(t, inner1Task.Id)
			assert.Equal(t, "inner", inner1.JobID)
			innerAttemptJobID := inner1.AttemptJobID
			callerAttempt1 := jobInLatest("caller")
			assert.True(t, callerAttempt1.IsExpanded)
			assert.Equal(t, callerAttempt1.ID, inner1.ParentJobID)
			runner.execTask(t, inner1Task, &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})

			run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runID})
			assert.Equal(t, actions_model.StatusSuccess, run.Status)

			// attempt 2: rerun gate, mock Failure -> caller stays unexpanded (Skipped)
			gateLatest := jobInLatest("gate")
			req := NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d/rerun", repo.OwnerName, repo.Name, runID, gateLatest.ID))
			user2Session.MakeRequest(t, req, http.StatusOK)

			gate2Task := runner.fetchTask(t)
			_, gate2, _ := getTaskAndJobAndRunByTaskID(t, gate2Task.Id)
			assert.Equal(t, "gate", gate2.JobID)
			runner.execTask(t, gate2Task, &mockTaskOutcome{result: runnerv1.Result_RESULT_FAILURE})

			runner.fetchNoTask(t) // no inner because caller did not expand
			attempt2 := latestAttempt()
			assert.Equal(t, actions_model.StatusFailure, attempt2.Status)
			callerAttempt2 := jobInLatest("caller")
			assert.Equal(t, actions_model.StatusSkipped, callerAttempt2.Status)
			assert.False(t, callerAttempt2.IsExpanded)
			assert.Equal(t, 0, unittest.GetCount(t, &actions_model.ActionRunJob{RunID: runID, RunAttemptID: attempt2.ID, JobID: "inner"}))

			// attempt 3: rerun gate, mock Success -> caller expands and inner reuses attempt 1's AttemptJobID
			gateLatest = jobInLatest("gate")
			req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d/rerun", repo.OwnerName, repo.Name, runID, gateLatest.ID))
			user2Session.MakeRequest(t, req, http.StatusOK)

			gate3Task := runner.fetchTask(t)
			runner.execTask(t, gate3Task, &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})

			inner3Task := runner.fetchTask(t)
			_, inner3, _ := getTaskAndJobAndRunByTaskID(t, inner3Task.Id)
			assert.Equal(t, "inner", inner3.JobID)
			assert.Equal(t, innerAttemptJobID, inner3.AttemptJobID)
			runner.execTask(t, inner3Task, &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})

			run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runID})
			assert.Equal(t, actions_model.StatusSuccess, run.Status)
		})
	})
}

// token must belong to u (the commit identity) and have write access to repo. Reuse the caller's
// existing token rather than logging in per call, which would re-run bcrypt password verification each time.
func createRepoWorkflowFile(t *testing.T, u *user_model.User, token string, repo *repo_model.Repository, treePath, content string) {
	opts := getWorkflowCreateFileOptions(u, repo.DefaultBranch, "create "+treePath, content)
	createWorkflowFile(t, token, repo.OwnerName, repo.Name, treePath, opts)
}

func getWorkflowCallPayloadFromTask(t *testing.T, runnerTask *runnerv1.Task) *api.WorkflowCallPayload {
	eventJSON, err := runnerTask.GetContext().Fields["event"].GetStructValue().MarshalJSON()
	assert.NoError(t, err)
	var payload api.WorkflowCallPayload
	assert.NoError(t, json.Unmarshal(eventJSON, &payload))
	return &payload
}
