// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"slices"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIActionsWorkflowRun(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()
	t.Run("GetWorkflowRun", testAPIActionsGetWorkflowRun)
	t.Run("GetWorkflowJob", testAPIActionsGetWorkflowJob)
	t.Run("ListUserWorkflows", testAPIActionsListUserWorkflows)
	t.Run("ListRepoWorkflows", testAPIActionsListRepoWorkflows)
	t.Run("DeleteRunCheckPermission", testAPIActionsDeleteRunCheckPermission)
	t.Run("DeleteRunRunning", testAPIActionsDeleteRunRunning)
	t.Run("DeleteRunGeneral", testAPIActionsDeleteRunGeneral)

	t.Run("RerunWorkflowRun", func(t *testing.T) {
		defer tests.PrepareTestEnv(t)()
		testAPIActionsRerunWorkflowRun(t)
	})
	t.Run("RerunWorkflowJob", func(t *testing.T) {
		defer tests.PrepareTestEnv(t)()
		testAPIActionsRerunWorkflowJob(t)
	})
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

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/jobs", repo.FullName())).AddTokenAuth(token)
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
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/jobs/198", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/jobs/196", repo.FullName())).
		AddTokenAuth(token)
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
	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/artifacts", repo.FullName())).AddTokenAuth(token)
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

func testAPIActionsRerunWorkflowRun(t *testing.T) {
	t.Run("NotDone", func(t *testing.T) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
		session := loginUser(t, user.Name)
		writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/793/rerun", repo.FullName())).
			AddTokenAuth(writeToken)
		MakeRequest(t, req, http.StatusBadRequest)
	})

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)

	writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	readToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	t.Run("Success", func(t *testing.T) {
		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/rerun", repo.FullName())).AddTokenAuth(writeToken)
		resp := MakeRequest(t, req, http.StatusCreated)
		rerunResp := DecodeJSON(t, resp, &api.ActionWorkflowRun{})

		assert.Equal(t, int64(795), rerunResp.ID)
		assert.Equal(t, "queued", rerunResp.Status)
		assert.Equal(t, "c2d72f548424103f01ee1dc02889c1e2bff816b0", rerunResp.HeadSha)

		run, err := actions_model.GetRunByRepoAndID(t.Context(), repo.ID, 795)
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
}

func testAPIActionsRerunWorkflowJob(t *testing.T) {
	t.Run("NotDone", func(t *testing.T) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
		session := loginUser(t, user.Name)
		writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/793/jobs/194/rerun", repo.FullName())).
			AddTokenAuth(writeToken)
		MakeRequest(t, req, http.StatusBadRequest)
	})

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)

	writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	readToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	t.Run("Success", func(t *testing.T) {
		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/jobs/199/rerun", repo.FullName())).AddTokenAuth(writeToken)
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

	t.Run("ForbiddenWithoutWriteScope", func(t *testing.T) {
		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/jobs/199/rerun", repo.FullName())).
			AddTokenAuth(readToken)
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("NotFoundJob", func(t *testing.T) {
		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/jobs/999999/rerun", repo.FullName())).
			AddTokenAuth(writeToken)
		MakeRequest(t, req, http.StatusNotFound)
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
