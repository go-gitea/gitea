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
	api "code.gitea.io/gitea/modules/structs"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"github.com/stretchr/testify/assert"
)

func TestActionsRerun(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		userAdmin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		sessionAdmin := loginUser(t, userAdmin.Name)

		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-rerun", false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		httpContext := NewAPITestContext(t, user2.Name, repo.Name, auth_model.AccessTokenScopeWriteRepository)
		defer doAPIDeleteRepository(httpContext)(t)

		runner := newMockRunner()
		runner.registerAsRepoRunner(t, repo.OwnerName, repo.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		wfTreePath := ".gitea/workflows/actions-rerun-workflow-1.yml"
		wfFileContent := `name: actions-rerun-workflow-1
on:
  push:
    paths:
      - '.gitea/workflows/actions-rerun-workflow-1.yml'
jobs:
  job1:
    runs-on: ubuntu-latest
    steps:
      - run: echo 'job1'
  job2:
    runs-on: ubuntu-latest
    needs: [job1]
    steps:
      - run: echo 'job2'
`

		opts := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, "create"+wfTreePath, wfFileContent)
		createWorkflowFile(t, token, user2.Name, repo.Name, wfTreePath, opts)

		// fetch and exec job1
		job1Task := runner.fetchTask(t)
		assert.Equal(t, "1", job1Task.Context.GetFields()["run_attempt"].GetStringValue())
		_, job1, run := getTaskAndJobAndRunByTaskID(t, job1Task.Id)
		runner.execTask(t, job1Task, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})
		// RERUN-FAILURE: the run is not done
		req := NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/rerun", user2.Name, repo.Name, run.ID))
		session.MakeRequest(t, req, http.StatusBadRequest)
		// fetch and exec job2
		job2Task := runner.fetchTask(t)
		_, job2, _ := getTaskAndJobAndRunByTaskID(t, job2Task.Id)
		runner.execTask(t, job2Task, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})
		assert.EqualValues(t, 1, getRunLatestAttemptNum(t, run.ID))

		// RERUN-1: rerun the run
		req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/rerun", user2.Name, repo.Name, run.ID))
		sessionAdmin.MakeRequest(t, req, http.StatusOK) // triggered by admin user
		// fetch and exec job1
		job1TaskR1 := runner.fetchTask(t)
		assert.Equal(t, "2", job1TaskR1.Context.GetFields()["run_attempt"].GetStringValue())
		_, job1R1, _ := getTaskAndJobAndRunByTaskID(t, job1TaskR1.Id)
		assert.Equal(t, job1.AttemptJobID, job1R1.AttemptJobID)
		runner.execTask(t, job1TaskR1, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})
		// fetch and exec job2
		job2TaskR1 := runner.fetchTask(t)
		assert.Equal(t, "2", job2TaskR1.Context.GetFields()["run_attempt"].GetStringValue())
		_, job2R1, _ := getTaskAndJobAndRunByTaskID(t, job2TaskR1.Id)
		assert.Equal(t, job2.AttemptJobID, job2R1.AttemptJobID)
		runner.execTask(t, job2TaskR1, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})
		assert.EqualValues(t, 2, getRunLatestAttemptNum(t, run.ID))

		// RERUN-2: rerun job1
		job1 = getLatestAttemptJobByTemplateJobID(t, run.ID, job1.ID)
		req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d/rerun", user2.Name, repo.Name, run.ID, job1.ID))
		session.MakeRequest(t, req, http.StatusOK)
		// job2 needs job1, so rerunning job1 will also rerun job2
		// fetch and exec job1
		job1TaskR2 := runner.fetchTask(t)
		assert.Equal(t, "3", job1TaskR2.Context.GetFields()["run_attempt"].GetStringValue())
		runner.execTask(t, job1TaskR2, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})
		// fetch and exec job2
		job2TaskR2 := runner.fetchTask(t)
		assert.Equal(t, "3", job2TaskR2.Context.GetFields()["run_attempt"].GetStringValue())
		runner.execTask(t, job2TaskR2, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})
		assert.EqualValues(t, 3, getRunLatestAttemptNum(t, run.ID))

		// RERUN-3: rerun job2
		job2 = getLatestAttemptJobByTemplateJobID(t, run.ID, job2.ID)
		req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d/rerun", user2.Name, repo.Name, run.ID, job2.ID))
		session.MakeRequest(t, req, http.StatusOK)
		// only job2 will rerun
		// fetch and exec job2
		job2TaskR3 := runner.fetchTask(t)
		assert.Equal(t, "4", job2TaskR3.Context.GetFields()["run_attempt"].GetStringValue())
		runner.execTask(t, job2TaskR3, &mockTaskOutcome{
			result: runnerv1.Result_RESULT_SUCCESS,
		})
		runner.fetchNoTask(t)
		assert.EqualValues(t, 4, getRunLatestAttemptNum(t, run.ID))

		runLatestAttempt := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
		job2LatestAttempt := getLatestAttemptJobByTemplateJobID(t, run.ID, job2.ID)
		assert.Equal(t, runLatestAttempt.LatestAttemptID, job2LatestAttempt.RunAttemptID)

		t.Run("AttemptAPI", func(t *testing.T) {
			req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/actions/runs/%d/attempts/2", user2.Name, repo.Name, run.ID)).
				AddTokenAuth(token)
			attemptResp := MakeRequest(t, req, http.StatusOK)
			apiAttempt := DecodeJSON(t, attemptResp, &api.ActionWorkflowRun{})
			assert.Equal(t, run.ID, apiAttempt.ID)
			assert.EqualValues(t, 2, apiAttempt.RunAttempt)
			assert.Equal(t, "completed", apiAttempt.Status)
			assert.Equal(t, "success", apiAttempt.Conclusion)
			assert.Equal(t, userAdmin.Name, apiAttempt.TriggerActor.UserName)

			req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/actions/runs/%d/attempts/2/jobs", user2.Name, repo.Name, run.ID)).
				AddTokenAuth(token)
			attemptJobsResp := MakeRequest(t, req, http.StatusOK)
			apiAttemptJobs := DecodeJSON(t, attemptJobsResp, &api.ActionWorkflowJobsResponse{})
			assert.Len(t, apiAttemptJobs.Entries, 2)
			assert.Equal(t, job1R1.ID, apiAttemptJobs.Entries[0].ID)
			assert.Equal(t, job2R1.ID, apiAttemptJobs.Entries[1].ID)
		})
	})
}

func getRunLatestAttemptNum(t *testing.T, runID int64) int64 {
	t.Helper()

	run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runID})
	attempt := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunAttempt{ID: run.LatestAttemptID})
	return attempt.Attempt
}
