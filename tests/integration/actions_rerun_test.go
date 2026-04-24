// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/actions/jobparser"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/timeutil"
	actions_web "code.gitea.io/gitea/routers/web/repo/actions"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			assert.NotNil(t, apiAttempt.PreviousAttemptURL)
			assert.True(t, strings.HasSuffix(*apiAttempt.PreviousAttemptURL, fmt.Sprintf("/api/v1/repos/%s/%s/actions/runs/%d/attempts/1", user2.Name, repo.Name, run.ID)))
			assert.Equal(t, user2.Name, apiAttempt.Actor.UserName)
			assert.Equal(t, userAdmin.Name, apiAttempt.TriggerActor.UserName)

			req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/actions/runs/%d/attempts/2/jobs", user2.Name, repo.Name, run.ID)).
				AddTokenAuth(token)
			attemptJobsResp := MakeRequest(t, req, http.StatusOK)
			apiAttemptJobs := DecodeJSON(t, attemptJobsResp, &api.ActionWorkflowJobsResponse{})
			assert.Len(t, apiAttemptJobs.Entries, 2)
			assert.ElementsMatch(t, []int64{job1R1.ID, job2R1.ID}, []int64{apiAttemptJobs.Entries[0].ID, apiAttemptJobs.Entries[1].ID})
		})

		t.Run("MaxRerunAttempts", func(t *testing.T) {
			// The run has 4 attempts after the previous reruns. Lower the cap to 4 to hit the limit.
			defer test.MockVariableValue(&setting.Actions.MaxRerunAttempts, int64(4))()

			req := NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/rerun", user2.Name, repo.Name, run.ID))
			resp := session.MakeRequest(t, req, http.StatusBadRequest)
			assert.Contains(t, resp.Body.String(), "workflow run has reached the maximum")
			assert.EqualValues(t, 4, getRunLatestAttemptNum(t, run.ID))

			// Raising the cap lets rerun proceed again.
			defer test.MockVariableValue(&setting.Actions.MaxRerunAttempts, int64(5))()

			req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/rerun", user2.Name, repo.Name, run.ID))
			session.MakeRequest(t, req, http.StatusOK)
			// fetch and exec job1
			job1TaskR4 := runner.fetchTask(t)
			assert.Equal(t, "5", job1TaskR4.Context.GetFields()["run_attempt"].GetStringValue())
			runner.execTask(t, job1TaskR4, &mockTaskOutcome{
				result: runnerv1.Result_RESULT_SUCCESS,
			})
			job2TaskR4 := runner.fetchTask(t)
			assert.Equal(t, "5", job2TaskR4.Context.GetFields()["run_attempt"].GetStringValue())
			runner.execTask(t, job2TaskR4, &mockTaskOutcome{
				result: runnerv1.Result_RESULT_SUCCESS,
			})
			assert.EqualValues(t, 5, getRunLatestAttemptNum(t, run.ID))
		})
	})
}

func TestActionsRerunLegacyNoAttemptRun(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		session := loginUser(t, user2.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiRepo := createActionsTestRepo(t, token, "actions-rerun-legacy", false)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		httpContext := NewAPITestContext(t, user2.Name, repo.Name, auth_model.AccessTokenScopeWriteRepository)
		defer doAPIDeleteRepository(httpContext)(t)
		runner := newMockRunner()
		runner.registerAsRepoRunner(t, repo.OwnerName, repo.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		wfTreePath := ".gitea/workflows/actions-rerun-legacy.yml"
		wfFileContent := `name: actions-rerun-legacy
on:
  workflow_dispatch:
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

		opts := getWorkflowCreateFileOptions(user2, repo.DefaultBranch, "create "+wfTreePath, wfFileContent)
		fileResp := createWorkflowFile(t, token, user2.Name, repo.Name, wfTreePath, opts)
		require.NotNil(t, fileResp)

		// Start preparing legacy data

		payloads := mustParseSingleWorkflowPayloads(t, wfFileContent)
		now := timeutil.TimeStamp(time.Now().Unix())
		started := now - 20
		stopped := now - 10

		legacyRun := &actions_model.ActionRun{
			Title:         "legacy rerun test",
			RepoID:        repo.ID,
			OwnerID:       repo.OwnerID,
			WorkflowID:    "actions-rerun-legacy.yml",
			Index:         1,
			TriggerUserID: user2.ID,
			Ref:           "refs/heads/" + repo.DefaultBranch,
			CommitSHA:     fileResp.Commit.SHA,
			Event:         "workflow_dispatch",
			TriggerEvent:  "workflow_dispatch",
			EventPayload:  "{}",
			Status:        actions_model.StatusSuccess,
			Started:       started,
			Stopped:       stopped,
			Created:       started - 5,
			Updated:       stopped,
		}
		require.NoError(t, db.Insert(t.Context(), legacyRun))
		// xorm does not update "created"-tagged fields via ORM methods; use raw SQL to backfill historical timestamps.
		_, err := db.GetEngine(t.Context()).Exec("UPDATE action_run SET created=?, updated=? WHERE id=?", int64(started-5), int64(stopped), legacyRun.ID)
		require.NoError(t, err)
		legacyRun.Created = started - 5
		legacyRun.Updated = stopped

		legacyJob1 := &actions_model.ActionRunJob{
			RunID:             legacyRun.ID,
			RepoID:            repo.ID,
			OwnerID:           repo.OwnerID,
			CommitSHA:         legacyRun.CommitSHA,
			Name:              payloads["job1"].name,
			Attempt:           1,
			WorkflowPayload:   payloads["job1"].payload,
			JobID:             "job1",
			Needs:             payloads["job1"].needs,
			RunsOn:            payloads["job1"].runsOn,
			Status:            actions_model.StatusSuccess,
			RunAttemptID:      0,
			AttemptJobID:      0,
			Started:           started,
			Stopped:           stopped,
			IsForkPullRequest: false,
		}
		legacyJob2 := &actions_model.ActionRunJob{
			RunID:             legacyRun.ID,
			RepoID:            repo.ID,
			OwnerID:           repo.OwnerID,
			CommitSHA:         legacyRun.CommitSHA,
			Name:              payloads["job2"].name,
			Attempt:           1,
			WorkflowPayload:   payloads["job2"].payload,
			JobID:             "job2",
			Needs:             payloads["job2"].needs,
			RunsOn:            payloads["job2"].runsOn,
			Status:            actions_model.StatusSuccess,
			RunAttemptID:      0,
			AttemptJobID:      0,
			Started:           started,
			Stopped:           stopped,
			IsForkPullRequest: false,
		}
		require.NoError(t, db.Insert(t.Context(), legacyJob1, legacyJob2))

		legacyTask1 := &actions_model.ActionTask{
			JobID:             legacyJob1.ID,
			Attempt:           1,
			Status:            actions_model.StatusSuccess,
			Started:           started,
			Stopped:           stopped,
			RepoID:            repo.ID,
			OwnerID:           repo.OwnerID,
			CommitSHA:         legacyRun.CommitSHA,
			IsForkPullRequest: false,
		}
		legacyTask1.GenerateAndFillToken()
		legacyTask2 := &actions_model.ActionTask{
			JobID:             legacyJob2.ID,
			Attempt:           1,
			Status:            actions_model.StatusSuccess,
			Started:           started,
			Stopped:           stopped,
			RepoID:            repo.ID,
			OwnerID:           repo.OwnerID,
			CommitSHA:         legacyRun.CommitSHA,
			IsForkPullRequest: false,
		}
		legacyTask2.GenerateAndFillToken()
		require.NoError(t, db.Insert(t.Context(), legacyTask1, legacyTask2))

		legacyJob1.TaskID = legacyTask1.ID
		legacyJob2.TaskID = legacyTask2.ID
		_, err = db.GetEngine(t.Context()).ID(legacyJob1.ID).Cols("task_id").Update(legacyJob1)
		require.NoError(t, err)
		_, err = db.GetEngine(t.Context()).ID(legacyJob2.ID).Cols("task_id").Update(legacyJob2)
		require.NoError(t, err)

		legacyArtifact := &actions_model.ActionArtifact{
			RunID:                 legacyRun.ID,
			RunAttemptID:          0,
			RepoID:                repo.ID,
			OwnerID:               repo.OwnerID,
			CommitSHA:             legacyRun.CommitSHA,
			StoragePath:           "artifacts/legacy-artifact.zip",
			FileSize:              123,
			FileCompressedSize:    123,
			ContentEncodingOrType: actions_model.ContentTypeZip,
			ArtifactPath:          "legacy-artifact.zip",
			ArtifactName:          "legacy-artifact",
			Status:                actions_model.ArtifactStatusUploadConfirmed,
			ExpiredUnix:           now + timeutil.Day,
		}
		require.NoError(t, db.Insert(t.Context(), legacyArtifact))

		// Done preparing legacy data

		// assert the web view for the legacy run before rerun
		req := NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d", user2.Name, repo.Name, legacyRun.ID))
		legacyResp := session.MakeRequest(t, req, http.StatusOK)
		legacyView := DecodeJSON(t, legacyResp, &actions_web.ViewResponse{})
		// legacy run has no attempt records, so RunAttempt is 0 and Attempts list is empty
		assert.EqualValues(t, 0, legacyView.State.Run.RunAttempt)
		assert.Empty(t, legacyView.State.Run.Attempts)
		assert.Equal(t, "success", legacyView.State.Run.Status)
		assert.True(t, legacyView.State.Run.Done)
		// isLatestAttempt=true, done=true: can rerun but not cancel
		assert.False(t, legacyView.State.Run.CanCancel)
		assert.False(t, legacyView.State.Run.CanApprove)
		assert.True(t, legacyView.State.Run.CanRerun)
		assert.False(t, legacyView.State.Run.CanRerunFailed) // all jobs succeeded
		assert.True(t, legacyView.State.Run.CanDeleteArtifact)
		if assert.Len(t, legacyView.State.Run.Jobs, 2) {
			assert.Equal(t, legacyJob1.ID, legacyView.State.Run.Jobs[0].ID)
			assert.Equal(t, legacyJob2.ID, legacyView.State.Run.Jobs[1].ID)
		}
		if assert.Len(t, legacyView.Artifacts, 1) {
			assert.Equal(t, legacyArtifact.ArtifactName, legacyView.Artifacts[0].Name)
			assert.Equal(t, "completed", legacyView.Artifacts[0].Status)
		}

		// rerun the legacy run
		req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/rerun", user2.Name, repo.Name, legacyRun.ID))
		session.MakeRequest(t, req, http.StatusOK)
		runAfterRerun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: legacyRun.ID})
		assert.EqualValues(t, 2, getRunLatestAttemptNum(t, legacyRun.ID))
		jobsAfterRerun, err := actions_model.GetRunJobsByRunAndAttemptID(t.Context(), legacyRun.ID, runAfterRerun.LatestAttemptID)
		require.NoError(t, err)
		require.Len(t, jobsAfterRerun, 2)
		rerunJobsByJobID := map[string]*actions_model.ActionRunJob{}
		for _, job := range jobsAfterRerun {
			rerunJobsByJobID[job.JobID] = job
		}
		require.Contains(t, rerunJobsByJobID, "job1")
		require.Contains(t, rerunJobsByJobID, "job2")
		assert.Equal(t, actions_model.StatusWaiting, rerunJobsByJobID["job1"].Status)
		assert.Equal(t, actions_model.StatusBlocked, rerunJobsByJobID["job2"].Status)

		// fetch job1 rerun task
		job1TaskR1 := runner.fetchTask(t)
		assert.Equal(t, "2", job1TaskR1.Context.GetFields()["run_attempt"].GetStringValue())
		rerunJob1Task, rerunJob1, rerunRun := getTaskAndJobAndRunByTaskID(t, job1TaskR1.Id)
		assert.Equal(t, legacyRun.ID, rerunRun.ID)
		assert.Equal(t, rerunJob1.RunAttemptID, rerunRun.LatestAttemptID)
		runner.execTask(t, job1TaskR1, &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})

		// fetch job2 rerun task
		job2TaskR1 := runner.fetchTask(t)
		assert.Equal(t, "2", job2TaskR1.Context.GetFields()["run_attempt"].GetStringValue())
		rerunJob2Task, rerunJob2, _ := getTaskAndJobAndRunByTaskID(t, job2TaskR1.Id)
		runner.execTask(t, job2TaskR1, &mockTaskOutcome{result: runnerv1.Result_RESULT_SUCCESS})
		runner.fetchNoTask(t)

		// query the 2 attempts
		runAfterRerun = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: legacyRun.ID})
		attempt1, err := actions_model.GetRunAttemptByRunIDAndAttemptNum(t.Context(), legacyRun.ID, 1)
		require.NoError(t, err)
		assert.Equal(t, legacyRun.Created, attempt1.Created)
		assert.Equal(t, legacyRun.Started, attempt1.Started)
		assert.Equal(t, legacyRun.Stopped, attempt1.Stopped)
		attempt2, err := actions_model.GetRunAttemptByRunIDAndAttemptNum(t.Context(), legacyRun.ID, 2)
		require.NoError(t, err)
		assert.Equal(t, attempt2.ID, runAfterRerun.LatestAttemptID)
		assert.Equal(t, runAfterRerun.Created, attempt1.Created)
		assert.Equal(t, runAfterRerun.Started, attempt2.Started)
		assert.Equal(t, runAfterRerun.Stopped, attempt2.Stopped)

		// assert legacy jobs
		legacyJob1 = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: legacyJob1.ID})
		legacyJob2 = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: legacyJob2.ID})
		assert.Equal(t, attempt1.ID, legacyJob1.RunAttemptID)
		assert.Equal(t, attempt1.ID, legacyJob2.RunAttemptID)
		assert.EqualValues(t, 1, legacyJob1.Attempt)
		assert.EqualValues(t, 1, legacyJob2.Attempt)
		assert.EqualValues(t, 1, legacyJob1.AttemptJobID)
		assert.EqualValues(t, 2, legacyJob2.AttemptJobID)
		legacyTask1 = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: legacyTask1.ID})
		legacyTask2 = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: legacyTask2.ID})
		assert.EqualValues(t, 1, legacyTask1.Attempt)
		assert.EqualValues(t, 1, legacyTask2.Attempt)

		// assert legacy artifacts
		legacyArtifact = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionArtifact{ID: legacyArtifact.ID})
		assert.Equal(t, attempt1.ID, legacyArtifact.RunAttemptID)

		// assert jobs of the latest rerun
		rerunJob1 = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: rerunJob1.ID})
		rerunJob2 = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: rerunJob2.ID})
		assert.Equal(t, attempt2.ID, rerunJob1.RunAttemptID)
		assert.Equal(t, attempt2.ID, rerunJob2.RunAttemptID)
		assert.Equal(t, legacyJob1.AttemptJobID, rerunJob1.AttemptJobID)
		assert.Equal(t, legacyJob2.AttemptJobID, rerunJob2.AttemptJobID)
		assert.EqualValues(t, 2, rerunJob1Task.Attempt)
		assert.EqualValues(t, 2, rerunJob2Task.Attempt)

		// assert the web view for the original attempt
		req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/attempts/1", user2.Name, repo.Name, legacyRun.ID))
		attempt1Resp := session.MakeRequest(t, req, http.StatusOK)
		attempt1View := DecodeJSON(t, attempt1Resp, &actions_web.ViewResponse{})
		assert.EqualValues(t, 1, attempt1View.State.Run.RunAttempt)
		if assert.Len(t, attempt1View.State.Run.Attempts, 2) {
			// attempts ordered by attempt DESC: index 0 = attempt #2 (latest), index 1 = attempt #1 (current)
			assert.False(t, attempt1View.State.Run.Attempts[0].Current)
			assert.True(t, attempt1View.State.Run.Attempts[0].Latest)
			assert.True(t, attempt1View.State.Run.Attempts[1].Current)
			assert.False(t, attempt1View.State.Run.Attempts[1].Latest)
		}
		// isLatestAttempt=false: all write operations disabled
		assert.False(t, attempt1View.State.Run.CanCancel)
		assert.False(t, attempt1View.State.Run.CanApprove)
		assert.False(t, attempt1View.State.Run.CanRerun)
		assert.False(t, attempt1View.State.Run.CanRerunFailed)
		assert.True(t, attempt1View.State.Run.CanDeleteArtifact)
		assert.Equal(t, legacyJob1.ID, attempt1View.State.Run.Jobs[0].ID)
		assert.Equal(t, legacyJob2.ID, attempt1View.State.Run.Jobs[1].ID)
		if assert.Len(t, attempt1View.Artifacts, 1) {
			assert.Equal(t, attempt1View.Artifacts[0].Name, legacyArtifact.ArtifactName)
		}

		// assert the web view for the latest attempt
		req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d", user2.Name, repo.Name, legacyRun.ID))
		attempt2Resp := session.MakeRequest(t, req, http.StatusOK)
		attempt2View := DecodeJSON(t, attempt2Resp, &actions_web.ViewResponse{})
		assert.EqualValues(t, 2, attempt2View.State.Run.RunAttempt)
		if assert.Len(t, attempt2View.State.Run.Attempts, 2) {
			// attempts ordered by attempt DESC: index 0 = attempt #2 (latest, current), index 1 = attempt #1
			assert.True(t, attempt2View.State.Run.Attempts[0].Current)
			assert.True(t, attempt2View.State.Run.Attempts[0].Latest)
			assert.False(t, attempt2View.State.Run.Attempts[1].Current)
			assert.False(t, attempt2View.State.Run.Attempts[1].Latest)
		}
		// isLatestAttempt=true, done=true: can rerun but not cancel
		assert.False(t, attempt2View.State.Run.CanCancel)
		assert.False(t, attempt2View.State.Run.CanApprove)
		assert.True(t, attempt2View.State.Run.CanRerun)
		assert.False(t, attempt2View.State.Run.CanRerunFailed) // all jobs succeeded
		assert.True(t, attempt2View.State.Run.CanDeleteArtifact)
		assert.Equal(t, rerunJob1.ID, attempt2View.State.Run.Jobs[0].ID)
		assert.Equal(t, rerunJob2.ID, attempt2View.State.Run.Jobs[1].ID)
		assert.Empty(t, attempt2View.Artifacts)
	})
}

type workflowJobPayload struct {
	name    string
	payload []byte
	needs   []string
	runsOn  []string
}

func mustParseSingleWorkflowPayloads(t *testing.T, workflowContent string) map[string]workflowJobPayload {
	t.Helper()

	workflows, err := jobparser.Parse([]byte(workflowContent))
	require.NoError(t, err)

	payloads := make(map[string]workflowJobPayload, len(workflows))
	for _, workflow := range workflows {
		id, job := workflow.Job()
		needs := job.Needs()
		require.NoError(t, workflow.SetJob(id, job.EraseNeeds()))
		payload, err := workflow.Marshal()
		require.NoError(t, err)
		payloads[id] = workflowJobPayload{
			name:    job.Name,
			payload: payload,
			needs:   needs,
			runsOn:  job.RunsOn(),
		}
	}
	return payloads
}

func getRunLatestAttemptNum(t *testing.T, runID int64) int64 {
	t.Helper()

	run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runID})
	attempt := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunAttempt{ID: run.LatestAttemptID})
	return attempt.Attempt
}
