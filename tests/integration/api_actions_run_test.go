// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"slices"
	"testing"
	"time"

	runnerv1 "gitea.dev/actions-proto-go/runner/v1"
	actions_model "gitea.dev/models/actions"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/actions"
	"gitea.dev/modules/commitstatus"
	"gitea.dev/modules/json"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/timeutil"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestAPIActionsWorkflowRun(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()
	t.Run("GetWorkflowRun", testAPIActionsGetWorkflowRun)
	t.Run("GetWorkflowJob", testAPIActionsGetWorkflowJob)
	t.Run("ListUserWorkflows", testAPIActionsListUserWorkflows)
	t.Run("ListRepoWorkflows", testAPIActionsListRepoWorkflows)
	t.Run("DeleteRunCheckPermission", testAPIActionsDeleteRunCheckPermission)
	t.Run("DeleteRunRunning", testAPIActionsDeleteRunRunning)
	t.Run("GetWorkflowRunLogsNotFound", testAPIActionsGetWorkflowRunLogsNotFound)
	t.Run("GetWorkflowJobLogsNotFound", testAPIActionsGetWorkflowJobLogsNotFound)
	// finishes run 793, so it must come after everything that needs it still running
	t.Run("CancelWorkflowRun", testAPIActionsCancelWorkflowRun)
	t.Run("ForceCancelWorkflowRun", testAPIActionsForceCancelWorkflowRun)
	t.Run("ApproveWorkflowRun", testAPIActionsApproveWorkflowRun)
	// deletes run 795, so it must come after everything that reads it
	t.Run("DeleteRunGeneral", testAPIActionsDeleteRunGeneral)
}

func testAPIActionsGetWorkflowRun(t *testing.T) {
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	t.Run("GetRun", func(t *testing.T) {
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/802802", repo.FullName())).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/802", repo.FullName())).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/803", repo.FullName())).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusOK)
	})

	t.Run("GetJobSteps", func(t *testing.T) {
		// Insert task steps for task_id 53 (job 198) so the API can return them once the backend loads them
		_, err := db.GetEngine(t.Context()).Insert(&actions_model.ActionTaskStep{
			Name:    "main",
			TaskID:  53,
			Index:   0,
			RepoID:  repo.ID,
			Status:  actions_model.StatusSuccess,
			Started: timeutil.TimeStamp(1683636528),
			Stopped: timeutil.TimeStamp(1683636626),
		})
		require.NoError(t, err)

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/jobs", repo.FullName())).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		jobList := DecodeJSON(t, resp, &api.ActionWorkflowJobsResponse{})

		job198Idx := slices.IndexFunc(jobList.Entries, func(job *api.ActionWorkflowJob) bool { return job.ID == 198 })
		require.NotEqual(t, -1, job198Idx, "expected to find job 198 in run 795 jobs list")
		job198 := jobList.Entries[job198Idx]
		require.NotEmpty(t, job198.Steps, "job must return at least one step when task has steps")
		assert.Equal(t, "main", job198.Steps[0].Name, "first step name")
	})
}

func testAPIActionsGetWorkflowJob(t *testing.T) {
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/jobs/198198", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/jobs/198", repo.FullName())).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/jobs/196", repo.FullName())).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func testAPIActionsDeleteRunCheckPermission(t *testing.T) {
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	testAPIActionsDeleteRun(t, repo, token, http.StatusNotFound)
}

func testAPIActionsDeleteRunGeneral(t *testing.T) {
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	testAPIActionsDeleteRunListArtifacts(t, repo, token, 2)
	testAPIActionsDeleteRunListTasks(t, repo, token, true)
	testAPIActionsDeleteRun(t, repo, token, http.StatusNoContent)

	testAPIActionsDeleteRunListArtifacts(t, repo, token, 0)
	testAPIActionsDeleteRunListTasks(t, repo, token, false)
	testAPIActionsDeleteRun(t, repo, token, http.StatusNotFound)
}

// needs run 793 still running, so it must come before CancelWorkflowRun
func testAPIActionsDeleteRunRunning(t *testing.T) {
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	req := NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/repos/%s/actions/runs/793", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusBadRequest)
}

func testAPIActionsDeleteRun(t *testing.T, repo *repo_model.Repository, token string, expected int) {
	req := NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, expected)
}

func testAPIActionsDeleteRunListArtifacts(t *testing.T, repo *repo_model.Repository, token string, artifacts int) {
	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/artifacts", repo.FullName())).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	listResp := DecodeJSON(t, resp, &api.ActionArtifactsResponse{})
	assert.Len(t, listResp.Entries, artifacts)
}

func testAPIActionsDeleteRunListTasks(t *testing.T, repo *repo_model.Repository, token string, expected bool) {
	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/tasks", repo.FullName())).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	listResp := DecodeJSON(t, resp, &api.ActionTaskResponse{})
	findTask1 := false
	findTask2 := false
	for _, entry := range listResp.Entries {
		if entry.ID == 53 {
			findTask1 = true
			continue
		}
		if entry.ID == 54 {
			findTask2 = true
			continue
		}
	}
	assert.Equal(t, expected, findTask1)
	assert.Equal(t, expected, findTask2)
}

// TestAPIActionsRerunWorkflowRun covers everything that mutates run 795, in a fixed order so
// they can share one fixture load: the log download has to see the original tasks, the job
// rerun needs the run still done, and the full rerun re-arms it by cancelling first.
func TestAPIActionsRerunWorkflowRun(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	t.Run("NotDone", func(t *testing.T) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
		session := loginUser(t, user.Name)
		writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/793/rerun", repo.FullName())).
			AddTokenAuth(writeToken)
		MakeRequest(t, req, http.StatusBadRequest)

		req = NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/793/jobs/194/rerun", repo.FullName())).
			AddTokenAuth(writeToken)
		MakeRequest(t, req, http.StatusBadRequest)
	})

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)

	writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	readToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	t.Run("RunLogs", func(t *testing.T) {
		// run 795 (workflow "test.yaml") has job 198 "job_1" on task 53 and job 199 "job_2" on task 54
		seedTaskLogs(t, 53, "hello from job_1")
		seedTaskLogs(t, 54, "hello from job_2")

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/logs", repo.FullName())).
			AddTokenAuth(writeToken)
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, "application/zip", resp.Header().Get("Content-Type"))
		assert.Contains(t, resp.Header().Get("Content-Disposition"), "test-run-795-logs.zip")
		assert.Equal(t, "Content-Disposition", resp.Header().Get("Access-Control-Expose-Headers"))

		body := resp.Body.Bytes()
		archive, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
		require.NoError(t, err)

		contents := make(map[string]string, len(archive.File))
		for _, file := range archive.File {
			r, err := file.Open()
			require.NoError(t, err)
			content, err := io.ReadAll(r)
			require.NoError(t, r.Close())
			require.NoError(t, err)
			contents[file.Name] = string(content)
		}

		require.Len(t, contents, 2)
		assert.Contains(t, contents["test-job_1-53.log"], "hello from job_1")
		assert.Contains(t, contents["test-job_2-54.log"], "hello from job_2")
	})

	t.Run("JobSuccess", func(t *testing.T) {
		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/jobs/199/rerun", repo.FullName())).
			AddTokenAuth(writeToken)
		resp := MakeRequest(t, req, http.StatusCreated)

		rerunResp := DecodeJSON(t, resp, &api.ActionWorkflowJob{})
		job199Rerun := getLatestAttemptJobByTemplateJobID(t, 795, 199)
		assert.Equal(t, job199Rerun.ID, rerunResp.ID)
		assert.Equal(t, "queued", rerunResp.Status)

		run, err := actions_model.GetRunByRepoAndID(t.Context(), repo.ID, 795)
		require.NoError(t, err)
		assert.Equal(t, actions_model.StatusWaiting, run.Status)
		latestAttempt, hasLatestAttempt, err := run.GetLatestAttempt(t.Context())
		require.NoError(t, err)
		require.True(t, hasLatestAttempt)

		job198Rerun := getLatestAttemptJobByTemplateJobID(t, 795, 198)
		assert.Equal(t, actions_model.StatusSuccess, job198Rerun.Status)
		assert.Equal(t, latestAttempt.Attempt, job198Rerun.Attempt)
		assert.Equal(t, int64(0), job198Rerun.TaskID)
		assert.Equal(t, int64(53), job198Rerun.SourceTaskID)

		job199Rerun = getLatestAttemptJobByTemplateJobID(t, 795, 199)
		assert.Equal(t, actions_model.StatusWaiting, job199Rerun.Status)
		assert.Equal(t, latestAttempt.Attempt, job199Rerun.Attempt)
		assert.Equal(t, int64(0), job199Rerun.TaskID)
		assert.Equal(t, int64(0), job199Rerun.SourceTaskID)
	})

	t.Run("Success", func(t *testing.T) {
		// JobSuccess above leaves the run waiting, so finish it to make it rerunnable again.
		// Run on its own the fixture run is still done and needs no cancelling.
		run, err := actions_model.GetRunByRepoAndID(t.Context(), repo.ID, 795)
		require.NoError(t, err)
		if !run.Status.IsDone() {
			req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/cancel", repo.FullName())).
				AddTokenAuth(writeToken)
			MakeRequest(t, req, http.StatusOK)
		}

		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/rerun", repo.FullName())).
			AddTokenAuth(writeToken)
		resp := MakeRequest(t, req, http.StatusCreated)

		rerunResp := DecodeJSON(t, resp, &api.ActionWorkflowRun{})
		assert.Equal(t, int64(795), rerunResp.ID)
		assert.Equal(t, "queued", rerunResp.Status)
		assert.Equal(t, "c2d72f548424103f01ee1dc02889c1e2bff816b0", rerunResp.HeadSha)

		run, err = actions_model.GetRunByRepoAndID(t.Context(), repo.ID, 795)
		require.NoError(t, err)
		assert.Equal(t, actions_model.StatusWaiting, run.Status)
		assert.Equal(t, timeutil.TimeStamp(0), run.Started)
		assert.Equal(t, timeutil.TimeStamp(0), run.Stopped)
		latestAttempt, hasLatestAttempt, err := run.GetLatestAttempt(t.Context())
		require.NoError(t, err)
		require.True(t, hasLatestAttempt)

		job198 := getLatestAttemptJobByTemplateJobID(t, 795, 198)
		assert.Equal(t, actions_model.StatusWaiting, job198.Status)
		assert.Equal(t, latestAttempt.Attempt, job198.Attempt)
		assert.Equal(t, int64(0), job198.TaskID)

		job199 := getLatestAttemptJobByTemplateJobID(t, 795, 199)
		assert.Equal(t, actions_model.StatusWaiting, job199.Status)
		assert.Equal(t, latestAttempt.Attempt, job199.Attempt)
		assert.Equal(t, int64(0), job199.TaskID)
	})

	t.Run("ForbiddenWithoutWriteScope", func(t *testing.T) {
		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/rerun", repo.FullName())).
			AddTokenAuth(readToken)
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("NotFound", func(t *testing.T) {
		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/999999/rerun", repo.FullName())).
			AddTokenAuth(writeToken)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("NotFoundJob", func(t *testing.T) {
		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/jobs/999999/rerun", repo.FullName())).
			AddTokenAuth(writeToken)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("NoLogsAfterRerun", func(t *testing.T) {
		// the full rerun above cleared both TaskID and SourceTaskID on every latest-attempt job
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/logs", repo.FullName())).
			AddTokenAuth(writeToken)
		MakeRequest(t, req, http.StatusNotFound)
	})
}

func testAPIActionsCancelWorkflowRun(t *testing.T) {
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	ownerSession := loginUser(t, owner.Name)
	ownerToken := getTokenForLoggedInUser(t, ownerSession, auth_model.AccessTokenScopeWriteRepository)

	t.Run("Success", func(t *testing.T) {
		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/793/cancel", repo.FullName())).
			AddTokenAuth(ownerToken)
		resp := MakeRequest(t, req, http.StatusOK)
		cancelledRun := DecodeJSON(t, resp, &api.ActionWorkflowRun{})
		assert.Equal(t, int64(793), cancelledRun.ID)
		assert.Equal(t, "completed", cancelledRun.Status)
		assert.Equal(t, "cancelled", cancelledRun.Conclusion)
	})

	t.Run("AlreadyCompleted", func(t *testing.T) {
		// run 791 already succeeded, so there is nothing left to cancel
		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/791/cancel", repo.FullName())).
			AddTokenAuth(ownerToken)
		MakeRequest(t, req, http.StatusConflict)
	})

	t.Run("NotFound", func(t *testing.T) {
		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/999999/cancel", repo.FullName())).
			AddTokenAuth(ownerToken)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("ForbiddenWithoutPermission", func(t *testing.T) {
		// user2 is not the owner of repo4 (owned by user5)
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		user2Session := loginUser(t, user2.Name)
		user2Token := getTokenForLoggedInUser(t, user2Session, auth_model.AccessTokenScopeWriteRepository)

		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/793/cancel", repo.FullName())).
			AddTokenAuth(user2Token)
		MakeRequest(t, req, http.StatusForbidden)
	})
}

func testAPIActionsForceCancelWorkflowRun(t *testing.T) {
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	ownerSession := loginUser(t, owner.Name)
	ownerToken := getTokenForLoggedInUser(t, ownerSession, auth_model.AccessTokenScopeWriteRepository)

	// repo4's master head, so the run's commit statuses are created against a commit that exists
	const commitSHA = "c7cd3cd144e6d23c9d6f3d07e52b2c1a956e0338"
	eventPayload, err := json.Marshal(&api.PushPayload{HeadCommit: &api.PayloadCommit{ID: commitSHA}})
	require.NoError(t, err)

	// A running run whose runner advertises cancelling support and reports on time:
	// a normal cancel only starts the graceful cancelling handshake, so only a force-cancel finishes it.
	run := &actions_model.ActionRun{
		Title:         "force-cancel-test",
		RepoID:        repo.ID,
		OwnerID:       repo.OwnerID,
		WorkflowID:    "force-cancel.yaml",
		Index:         9601,
		TriggerUserID: owner.ID,
		Ref:           "refs/heads/master",
		CommitSHA:     commitSHA,
		Event:         "push",
		TriggerEvent:  "push",
		EventPayload:  string(eventPayload),
		Status:        actions_model.StatusRunning,
		Started:       timeutil.TimeStampNow(),
	}
	require.NoError(t, db.Insert(t.Context(), run))

	attempt := &actions_model.ActionRunAttempt{
		RepoID:        run.RepoID,
		RunID:         run.ID,
		Attempt:       1,
		TriggerUserID: owner.ID,
		Status:        actions_model.StatusRunning,
		Started:       timeutil.TimeStampNow(),
	}
	require.NoError(t, db.Insert(t.Context(), attempt))
	run.LatestAttemptID = attempt.ID
	require.NoError(t, actions_model.UpdateRun(t.Context(), run, "latest_attempt_id"))

	job := &actions_model.ActionRunJob{
		RunID:        run.ID,
		RunAttemptID: attempt.ID,
		RepoID:       run.RepoID,
		OwnerID:      run.OwnerID,
		CommitSHA:    run.CommitSHA,
		Name:         "job1",
		Attempt:      1,
		JobID:        "job1",
		Status:       actions_model.StatusRunning,
	}
	require.NoError(t, db.Insert(t.Context(), job))

	runner := &actions_model.ActionRunner{
		UUID:                 "force-cancel-runner",
		Name:                 "force-cancel-runner",
		RepoID:               repo.ID,
		HasCancellingSupport: true,
	}
	runner.GenerateAndFillToken()
	require.NoError(t, db.Insert(t.Context(), runner))

	task := &actions_model.ActionTask{
		JobID:     job.ID,
		Attempt:   1,
		RunnerID:  runner.ID,
		Status:    actions_model.StatusRunning,
		Started:   timeutil.TimeStampNow(),
		RepoID:    run.RepoID,
		OwnerID:   run.OwnerID,
		CommitSHA: run.CommitSHA,
	}
	require.NoError(t, db.Insert(t.Context(), task))

	job.TaskID = task.ID
	_, err = actions_model.UpdateRunJob(t.Context(), job, nil, "task_id")
	require.NoError(t, err)

	cancelURL := fmt.Sprintf("/api/v1/repos/%s/actions/runs/%d/cancel", repo.FullName(), run.ID)
	forceCancelURL := fmt.Sprintf("/api/v1/repos/%s/actions/runs/%d/force-cancel", repo.FullName(), run.ID)

	// a normal cancel only starts the graceful handshake
	MakeRequest(t, NewRequest(t, "POST", cancelURL).AddTokenAuth(ownerToken), http.StatusOK)
	cancellingTask := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: task.ID})
	assert.Equal(t, actions_model.StatusCancelling, cancellingTask.Status)

	// the commit status describes the cancellation, not the job's pre-cancel state
	statuses, err := git_model.GetLatestCommitStatus(t.Context(), repo.ID, commitSHA, db.ListOptionsAll)
	require.NoError(t, err)
	require.Len(t, statuses, 1)
	assert.Equal(t, "Canceling", statuses[0].Description)

	// force-cancel bypasses the handshake and finishes the run immediately
	resp := MakeRequest(t, NewRequest(t, "POST", forceCancelURL).AddTokenAuth(ownerToken), http.StatusOK)
	cancelledRun := DecodeJSON(t, resp, &api.ActionWorkflowRun{})
	assert.Equal(t, "completed", cancelledRun.Status)
	assert.Equal(t, "cancelled", cancelledRun.Conclusion)

	cancelledTask := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: task.ID})
	assert.Equal(t, actions_model.StatusCancelled, cancelledTask.Status)
	gotAttempt := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunAttempt{ID: attempt.ID})
	assert.Equal(t, actions_model.StatusCancelled, gotAttempt.Status)
	gotRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
	assert.Equal(t, actions_model.StatusCancelled, gotRun.Status)

	// the run is done, so its commit status must be final instead of pending
	statuses, err = git_model.GetLatestCommitStatus(t.Context(), repo.ID, commitSHA, db.ListOptionsAll)
	require.NoError(t, err)
	require.Len(t, statuses, 1)
	assert.Equal(t, commitstatus.CommitStatusFailure, statuses[0].State)

	// both endpoints refuse the completed run
	MakeRequest(t, NewRequest(t, "POST", cancelURL).AddTokenAuth(ownerToken), http.StatusConflict)
	MakeRequest(t, NewRequest(t, "POST", forceCancelURL).AddTokenAuth(ownerToken), http.StatusConflict)

	// the route is guarded like /cancel: user2 has no access to repo4, owned by user5
	user2Token := getTokenForLoggedInUser(t, loginUser(t, "user2"), auth_model.AccessTokenScopeWriteRepository)
	MakeRequest(t, NewRequest(t, "POST", forceCancelURL).AddTokenAuth(user2Token), http.StatusForbidden)

	missingRunURL := fmt.Sprintf("/api/v1/repos/%s/actions/runs/999999/force-cancel", repo.FullName())
	MakeRequest(t, NewRequest(t, "POST", missingRunURL).AddTokenAuth(ownerToken), http.StatusNotFound)
}

func testAPIActionsApproveWorkflowRun(t *testing.T) {
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	// user5 owns repo4, user4 is a write collaborator on it, user2 has no access at all
	ownerToken := getTokenForLoggedInUser(t, loginUser(t, "user5"), auth_model.AccessTokenScopeWriteRepository)
	writerToken := getTokenForLoggedInUser(t, loginUser(t, "user4"), auth_model.AccessTokenScopeWriteRepository)
	strangerToken := getTokenForLoggedInUser(t, loginUser(t, "user2"), auth_model.AccessTokenScopeWriteRepository)

	// a fork PR from a first-time contributor is what produces these in practice, which
	// actions_approve_test.go already covers end to end
	insertBlockedRun := func(index int64) *actions_model.ActionRun {
		run := &actions_model.ActionRun{
			Title: "needs approval", RepoID: repo.ID, OwnerID: repo.OwnerID, WorkflowID: "test.yaml", Index: index,
			TriggerUserID: 4, Ref: "refs/heads/main", CommitSHA: "c2d72f548424103f01ee1dc02889c1e2bff816b0",
			Event: "pull_request", TriggerEvent: "pull_request",
			Status: actions_model.StatusBlocked, NeedApproval: true,
		}
		require.NoError(t, db.Insert(t.Context(), run))
		require.NoError(t, db.Insert(t.Context(), &actions_model.ActionRunJob{
			RunID: run.ID, RepoID: run.RepoID, OwnerID: run.OwnerID, CommitSHA: run.CommitSHA,
			Name: "job1", Attempt: 1, JobID: "job1", Status: actions_model.StatusBlocked, RunsOn: []string{"ubuntu-latest"},
		}))
		return run
	}

	assertApproved := func(t *testing.T, runID, approverID int64) {
		t.Helper()
		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runID})
		assert.False(t, run.NeedApproval)
		assert.Equal(t, approverID, run.ApprovedBy)
		jobs, err := actions_model.GetLatestAttemptJobsByRun(t.Context(), run)
		require.NoError(t, err)
		for _, job := range jobs {
			assert.Equal(t, actions_model.StatusWaiting, job.Status)
		}
	}

	run := insertBlockedRun(2001)

	t.Run("ForbiddenWithoutPermission", func(t *testing.T) {
		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/%d/approve", repo.FullName(), run.ID)).
			AddTokenAuth(strangerToken)
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("AsOwner", func(t *testing.T) {
		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/%d/approve", repo.FullName(), run.ID)).
			AddTokenAuth(ownerToken)
		resp := MakeRequest(t, req, http.StatusOK)
		apiRun := DecodeJSON(t, resp, &api.ActionWorkflowRun{})
		assert.Equal(t, run.ID, apiRun.ID)
		assertApproved(t, run.ID, 5)
	})

	t.Run("AgainIsIdempotent", func(t *testing.T) {
		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/%d/approve", repo.FullName(), run.ID)).
			AddTokenAuth(ownerToken)
		MakeRequest(t, req, http.StatusOK)
		assertApproved(t, run.ID, 5)
	})

	t.Run("AsWriterNonAdmin", func(t *testing.T) {
		writerRun := insertBlockedRun(2002)
		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/%d/approve", repo.FullName(), writerRun.ID)).
			AddTokenAuth(writerToken)
		MakeRequest(t, req, http.StatusOK)
		assertApproved(t, writerRun.ID, 4)
	})

	t.Run("NotRequired", func(t *testing.T) {
		// run 791 succeeded without ever awaiting approval
		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/791/approve", repo.FullName())).
			AddTokenAuth(ownerToken)
		MakeRequest(t, req, http.StatusConflict)
	})
}

func testAPIActionsListUserWorkflows(t *testing.T) {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser)

	t.Run("Runs", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/user/actions/runs").AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		runs := DecodeJSON(t, resp, &api.ActionWorkflowRunsResponse{})

		assert.Positive(t, runs.TotalCount)
		assert.NotEmpty(t, runs.Entries)

		for _, run := range runs.Entries {
			assert.NotEmpty(t, run.DisplayTitle, "display_title should be populated")
			assert.NotNil(t, run.Repository, "repository should be populated via batch loading")
			assert.NotEmpty(t, run.Repository.FullName, "repository full_name should be populated")
			assert.NotNil(t, run.TriggerActor, "trigger_actor should be populated via batch loading")
		}
	})

	t.Run("Jobs", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/user/actions/jobs").AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		jobs := DecodeJSON(t, resp, &api.ActionWorkflowJobsResponse{})

		assert.Positive(t, jobs.TotalCount)
		assert.NotEmpty(t, jobs.Entries)

		for _, job := range jobs.Entries {
			assert.NotEmpty(t, job.Name, "job name should be populated")
			assert.NotEmpty(t, job.HTMLURL, "html_url should be populated via batch-loaded repo")
		}
	})

	t.Run("JobsDefaultOrderAsc", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/user/actions/jobs").AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		jobs := DecodeJSON(t, resp, &api.ActionWorkflowJobsResponse{})

		assert.GreaterOrEqual(t, len(jobs.Entries), 2, "need at least 2 jobs to verify ordering")
		for i := 1; i < len(jobs.Entries); i++ {
			assert.Less(t, jobs.Entries[i-1].ID, jobs.Entries[i].ID,
				"jobs should be ordered by ID ascending by default")
		}
	})

	t.Run("JobsOrderedByIDDesc", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/user/actions/jobs?sort=id&order=desc").AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		jobs := DecodeJSON(t, resp, &api.ActionWorkflowJobsResponse{})

		assert.GreaterOrEqual(t, len(jobs.Entries), 2, "need at least 2 jobs to verify ordering")
		for i := 1; i < len(jobs.Entries); i++ {
			assert.Greater(t, jobs.Entries[i-1].ID, jobs.Entries[i].ID,
				"jobs should be ordered by ID descending")
		}
	})
}

func testAPIActionsListRepoWorkflows(t *testing.T) {
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs", repo.FullName())).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	runs := DecodeJSON(t, resp, &api.ActionWorkflowRunsResponse{})

	assert.Positive(t, runs.TotalCount)
	assert.NotEmpty(t, runs.Entries)

	for _, run := range runs.Entries {
		assert.NotNil(t, run.Repository, "repository should be populated from ctx.Repo")
		assert.Equal(t, repo.FullName(), run.Repository.FullName, "repository full_name should match")
		assert.NotNil(t, run.TriggerActor, "trigger_actor should be populated")
	}
}

func testAPIActionsGetWorkflowRunLogsNotFound(t *testing.T) {
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	t.Run("NoLogs", func(t *testing.T) {
		// Run 795 has jobs but fixture tasks have no log output in storage.
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/logs", repo.FullName())).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("RunNotFound", func(t *testing.T) {
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/999999/logs", repo.FullName())).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})
}

// seedTaskLogs writes task logs the way the runner does, in DBFS to stay independent of the object storage fixture.
func seedTaskLogs(t *testing.T, taskID int64, lines ...string) {
	t.Helper()

	task, err := actions_model.GetTaskByID(t.Context(), taskID)
	require.NoError(t, err)

	task.LogInStorage = false
	task.LogFilename = fmt.Sprintf("test-logs/%d.log", task.ID)

	rows := make([]*runnerv1.LogRow, 0, len(lines))
	for _, line := range lines {
		rows = append(rows, &runnerv1.LogRow{Time: timestamppb.New(time.Unix(1683636528, 0)), Content: line})
	}
	ns, err := actions.WriteLogs(t.Context(), task.LogFilename, 0, rows)
	require.NoError(t, err)

	task.LogLength = int64(len(rows))
	for _, n := range ns {
		task.LogIndexes = append(task.LogIndexes, task.LogSize)
		task.LogSize += int64(n)
	}
	require.NoError(t, actions_model.UpdateTask(t.Context(), task,
		"log_filename", "log_in_storage", "log_indexes", "log_length", "log_size"))
}

// the success path is covered against real runner logs by TestDownloadTaskLogs
func testAPIActionsGetWorkflowJobLogsNotFound(t *testing.T) {
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	t.Run("NoLogFile", func(t *testing.T) {
		// job 199 exists but its task has no log file in the test fixture
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/jobs/199/logs", repo.FullName())).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("JobNotFound", func(t *testing.T) {
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/jobs/999999/logs", repo.FullName())).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})
}

// TestAPIOrgActionsRunsAccessControl ensures the org-level Actions run/job listing does not
// leak runs/jobs from repos the caller cannot access.
func TestAPIOrgActionsRunsAccessControl(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// org3 has action run 802 (and its jobs) in the private repo5; user28 is an org3 member
	// (teams 12/13) with no access to repo5.
	token := getUserToken(t, "user28", auth_model.AccessTokenScopeReadOrganization)

	req := NewRequest(t, "GET", "/api/v1/orgs/org3/actions/runs").AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	runs := DecodeJSON(t, resp, &api.ActionWorkflowRunsResponse{})
	for _, r := range runs.Entries {
		assert.NotEqual(t, int64(802), r.ID, "must not leak a run from an inaccessible repo")
	}

	req = NewRequest(t, "GET", "/api/v1/orgs/org3/actions/jobs").AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	jobs := DecodeJSON(t, resp, &api.ActionWorkflowJobsResponse{})
	for _, j := range jobs.Entries {
		assert.NotEqual(t, int64(802), j.RunID, "must not leak a job from an inaccessible repo run")
	}

	// user1 is a site admin: it normally bypasses the per-repo access filter, but a public-only token
	// must stay confined to public repos, so the run/job in the private repo5 must not be listed.
	adminPublicOnly := getUserToken(t, "user1", auth_model.AccessTokenScopeReadOrganization, auth_model.AccessTokenScopePublicOnly)

	req = NewRequest(t, "GET", "/api/v1/orgs/org3/actions/runs").AddTokenAuth(adminPublicOnly)
	resp = MakeRequest(t, req, http.StatusOK)
	adminRuns := DecodeJSON(t, resp, &api.ActionWorkflowRunsResponse{})
	for _, r := range adminRuns.Entries {
		assert.NotEqual(t, int64(802), r.ID, "a public-only admin token must not list a private repo's run")
	}

	req = NewRequest(t, "GET", "/api/v1/orgs/org3/actions/jobs").AddTokenAuth(adminPublicOnly)
	resp = MakeRequest(t, req, http.StatusOK)
	adminJobs := DecodeJSON(t, resp, &api.ActionWorkflowJobsResponse{})
	for _, j := range adminJobs.Entries {
		assert.NotEqual(t, int64(802), j.RunID, "a public-only admin token must not list a private repo's job")
	}
}
