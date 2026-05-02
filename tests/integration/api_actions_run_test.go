// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIActionsGetWorkflowRun(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

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

		var jobList api.ActionWorkflowJobsResponse
		err = json.Unmarshal(resp.Body.Bytes(), &jobList)
		require.NoError(t, err)

		job198Idx := slices.IndexFunc(jobList.Entries, func(job *api.ActionWorkflowJob) bool { return job.ID == 198 })
		require.NotEqual(t, -1, job198Idx, "expected to find job 198 in run 795 jobs list")
		job198 := jobList.Entries[job198Idx]
		require.NotEmpty(t, job198.Steps, "job must return at least one step when task has steps")
		assert.Equal(t, "main", job198.Steps[0].Name, "first step name")
	})
}

func TestAPIActionsGetWorkflowJob(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

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

func TestAPIActionsDeleteRunCheckPermission(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	testAPIActionsDeleteRun(t, repo, token, http.StatusNotFound)
}

func TestAPIActionsDeleteRun(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

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

func TestAPIActionsDeleteRunRunning(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

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
	var listResp api.ActionArtifactsResponse
	err := json.Unmarshal(resp.Body.Bytes(), &listResp)
	assert.NoError(t, err)
	assert.Len(t, listResp.Entries, artifacts)
}

func testAPIActionsDeleteRunListTasks(t *testing.T, repo *repo_model.Repository, token string, expected bool) {
	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/tasks", repo.FullName())).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	var listResp api.ActionTaskResponse
	err := json.Unmarshal(resp.Body.Bytes(), &listResp)
	assert.NoError(t, err)
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
	})

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)

	writeToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	readToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)

	t.Run("Success", func(t *testing.T) {
		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/rerun", repo.FullName())).
			AddTokenAuth(writeToken)
		resp := MakeRequest(t, req, http.StatusCreated)

		var rerunResp api.ActionWorkflowRun
		err := json.Unmarshal(resp.Body.Bytes(), &rerunResp)
		require.NoError(t, err)
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

func TestAPIActionsCancelWorkflowRun(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	ownerSession := loginUser(t, owner.Name)
	ownerToken := getTokenForLoggedInUser(t, ownerSession, auth_model.AccessTokenScopeWriteRepository)

	t.Run("Success", func(t *testing.T) {
		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/793/cancel", repo.FullName())).
			AddTokenAuth(ownerToken)
		MakeRequest(t, req, http.StatusOK)
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

func TestAPIActionsApproveWorkflowRun(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		// user2 is the owner of the base repo
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		user2Session := loginUser(t, user2.Name)
		user2Token := getTokenForLoggedInUser(t, user2Session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
		// user4 is the owner of the fork repo
		user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
		user4Token := getTokenForLoggedInUser(t, loginUser(t, user4.Name), auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		apiBaseRepo := createActionsTestRepo(t, user2Token, "approve-workflow-run", false)
		baseRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiBaseRepo.ID})
		user2APICtx := NewAPITestContext(t, baseRepo.OwnerName, baseRepo.Name, auth_model.AccessTokenScopeWriteRepository)
		defer doAPIDeleteRepository(user2APICtx)(t)

		runner := newMockRunner()
		runner.registerAsRepoRunner(t, baseRepo.OwnerName, baseRepo.Name, "mock-runner", []string{"ubuntu-latest"}, false)

		// init workflow
		wfTreePath := ".gitea/workflows/approve.yml"
		wfFileContent := `name: Approve
on: pull_request
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: echo test
`
		opts := getWorkflowCreateFileOptions(user2, baseRepo.DefaultBranch, "create %s"+wfTreePath, wfFileContent)
		createWorkflowFile(t, user2Token, baseRepo.OwnerName, baseRepo.Name, wfTreePath, opts)

		// user4 forks the repo
		forkName := "approve-workflow-run-fork"
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/forks", baseRepo.OwnerName, baseRepo.Name),
			&api.CreateForkOption{
				Name: &forkName,
			}).AddTokenAuth(user4Token)
		resp := MakeRequest(t, req, http.StatusAccepted)
		apiForkRepo := DecodeJSON(t, resp, &api.Repository{})
		forkRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiForkRepo.ID})
		user4APICtx := NewAPITestContext(t, user4.Name, forkRepo.Name, auth_model.AccessTokenScopeWriteRepository)
		defer doAPIDeleteRepository(user4APICtx)(t)

		// user4 creates a pull request from a branch
		doAPICreateFile(user4APICtx, "test.txt", &api.CreateFileOptions{
			FileOptions: api.FileOptions{
				NewBranchName: "feature/test",
				Message:       "create test.txt",
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
			ContentBase64: base64.StdEncoding.EncodeToString([]byte("test")),
		})(t)
		_, err := doAPICreatePullRequest(user4APICtx, baseRepo.OwnerName, baseRepo.Name, baseRepo.DefaultBranch, user4.Name+":feature/test")(t)
		assert.NoError(t, err)

		// check run
		run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: baseRepo.ID, TriggerUserID: user4.ID})
		assert.True(t, run.NeedApproval)
		assert.Equal(t, actions_model.StatusBlocked, run.Status)

		// Test approve workflow run via API
		req = NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/%d/approve", baseRepo.FullName(), run.ID)).
			AddTokenAuth(user2Token)
		MakeRequest(t, req, http.StatusOK)

		// Verify run was approved and jobs unblocked
		updatedRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
		assert.False(t, updatedRun.NeedApproval)
		assert.Equal(t, user2.ID, updatedRun.ApprovedBy)
		jobs, err := actions_model.GetLatestAttemptJobsByRepoAndRunID(t.Context(), baseRepo.ID, run.ID)
		require.NoError(t, err)
		for _, job := range jobs {
			assert.Equal(t, actions_model.StatusWaiting, job.Status)
		}

		// Test approve non-existent run
		req = NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/999999/approve", baseRepo.FullName())).
			AddTokenAuth(user2Token)
		MakeRequest(t, req, http.StatusNotFound)

		// Test approve by non-owner (user4 should get forbidden)
		req = NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/%d/approve", baseRepo.FullName(), run.ID)).
			AddTokenAuth(user4Token)
		MakeRequest(t, req, http.StatusForbidden)
	})
}

func TestAPIActionsRerunWorkflowJob(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

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
		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/jobs/199/rerun", repo.FullName())).
			AddTokenAuth(writeToken)
		resp := MakeRequest(t, req, http.StatusCreated)

		var rerunResp api.ActionWorkflowJob
		err := json.Unmarshal(resp.Body.Bytes(), &rerunResp)
		require.NoError(t, err)
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

func TestAPIActionsGetWorkflowRunLogs(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	t.Run("Success", func(t *testing.T) {
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/logs", repo.FullName())).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusOK)
	})

	t.Run("NotFound", func(t *testing.T) {
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/999999/logs", repo.FullName())).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})
}

func TestAPIActionsGetWorkflowJobLogs(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	t.Run("NoLogFile", func(t *testing.T) {
		// Job 198 exists but has no log file in the test fixture
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/jobs/198/logs", repo.FullName())).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("JobNotFound", func(t *testing.T) {
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/jobs/999999/logs", repo.FullName())).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})
}

func TestAPIActionsGetWorkflowRunLogsStream(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	t.Run("EmptyCursors", func(t *testing.T) {
		req := NewRequestWithBody(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/logs", repo.FullName()), strings.NewReader(`{"logCursors": []}`)).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		var logResp map[string]any
		err := json.Unmarshal(resp.Body.Bytes(), &logResp)
		assert.NoError(t, err)
		assert.Contains(t, logResp, "stepsLog")
	})

	t.Run("WithCursor", func(t *testing.T) {
		req := NewRequestWithBody(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/logs", repo.FullName()), strings.NewReader(`{"logCursors": [{"step": 0, "cursor": 0, "expanded": true}]}`)).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusOK)
	})

	t.Run("NotFound", func(t *testing.T) {
		req := NewRequestWithBody(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/999999/logs", repo.FullName()), strings.NewReader(`{"logCursors": []}`)).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNotFound)
	})
}
