// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"

	runnerv1 "gitea.dev/actions-proto-go/runner/v1"
	actions_model "gitea.dev/models/actions"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/actions"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/timeutil"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestAPIActionsWorkflowRun groups the read-only run/workflow endpoint tests so they
// share a single fixture reload; cases that mutate fixture rows reset the env themselves.
func TestAPIActionsWorkflowRun(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()
	t.Run("GetWorkflowRun", testAPIActionsGetWorkflowRun)
	t.Run("GetWorkflowJob", testAPIActionsGetWorkflowJob)
	t.Run("ListUserWorkflows", testAPIActionsListUserWorkflows)
	t.Run("ListRepoWorkflows", testAPIActionsListRepoWorkflows)
	t.Run("DeleteRunCheckPermission", testAPIActionsDeleteRunCheckPermission)
	t.Run("DeleteRunRunning", testAPIActionsDeleteRunRunning)
	t.Run("GetWorkflowRunLogsNotFound", testAPIActionsGetWorkflowRunLogsNotFound)
	t.Run("ApproveRunNotRequired", testAPIActionsApproveRunNotRequired)
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
	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/artifacts", repo.FullName())).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	listResp := DecodeJSON(t, resp, &api.ActionArtifactsResponse{})
	assert.Len(t, listResp.Entries, artifacts)
}

func testAPIActionsDeleteRunListTasks(t *testing.T, repo *repo_model.Repository, token string, expected bool) {
	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/tasks", repo.FullName())).
		AddTokenAuth(token)
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

func TestAPIActionsCancelWorkflowRun(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

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

		// init two workflows so a second run stays blocked after the first is approved,
		// which lets the writer-but-non-admin case below actually exercise the approval
		wf1TreePath := ".gitea/workflows/approve_1.yml"
		wf1FileContent := `name: Approve 1
on: pull_request
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: echo test
`
		opts1 := getWorkflowCreateFileOptions(user2, baseRepo.DefaultBranch, "create %s"+wf1TreePath, wf1FileContent)
		createWorkflowFile(t, user2Token, baseRepo.OwnerName, baseRepo.Name, wf1TreePath, opts1)
		wf2TreePath := ".gitea/workflows/approve_2.yml"
		wf2FileContent := `name: Approve 2
on: pull_request
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: echo test
`
		opts2 := getWorkflowCreateFileOptions(user2, baseRepo.DefaultBranch, "create %s"+wf2TreePath, wf2FileContent)
		createWorkflowFile(t, user2Token, baseRepo.OwnerName, baseRepo.Name, wf2TreePath, opts2)

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

		// check runs
		run1 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: baseRepo.ID, TriggerUserID: user4.ID, WorkflowID: "approve_1.yml"})
		assert.True(t, run1.NeedApproval)
		assert.Equal(t, actions_model.StatusBlocked, run1.Status)
		run2 := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{RepoID: baseRepo.ID, TriggerUserID: user4.ID, WorkflowID: "approve_2.yml"})
		assert.True(t, run2.NeedApproval)
		assert.Equal(t, actions_model.StatusBlocked, run2.Status)

		assertApproved := func(t *testing.T, runID, approverID int64) {
			t.Helper()
			approvedRun := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: runID})
			assert.False(t, approvedRun.NeedApproval)
			assert.Equal(t, approverID, approvedRun.ApprovedBy)
			jobs, err := actions_model.GetLatestAttemptJobsByRepoAndRunID(t.Context(), baseRepo.ID, runID)
			require.NoError(t, err)
			for _, job := range jobs {
				assert.Equal(t, actions_model.StatusWaiting, job.Status)
			}
		}

		t.Run("ApproveAsOwner", func(t *testing.T) {
			req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/%d/approve", baseRepo.FullName(), run1.ID)).
				AddTokenAuth(user2Token)
			resp := MakeRequest(t, req, http.StatusOK)
			apiRun := DecodeJSON(t, resp, &api.ActionWorkflowRun{})
			assert.Equal(t, run1.ID, apiRun.ID)
			assert.NotEqual(t, "waiting", apiRun.Status, "approved run should not be blocked")
			assertApproved(t, run1.ID, user2.ID)
		})

		t.Run("ApproveAgainIsIdempotent", func(t *testing.T) {
			req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/%d/approve", baseRepo.FullName(), run1.ID)).
				AddTokenAuth(user2Token)
			resp := MakeRequest(t, req, http.StatusOK)
			apiRun := DecodeJSON(t, resp, &api.ActionWorkflowRun{})
			assert.NotEqual(t, "waiting", apiRun.Status, "already-approved run should not be blocked")
			assertApproved(t, run1.ID, user2.ID)
		})

		t.Run("RunNotFound", func(t *testing.T) {
			req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/999999/approve", baseRepo.FullName())).
				AddTokenAuth(user2Token)
			MakeRequest(t, req, http.StatusNotFound)
		})

		t.Run("ForbiddenWithoutPermission", func(t *testing.T) {
			req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/%d/approve", baseRepo.FullName(), run2.ID)).
				AddTokenAuth(user4Token)
			MakeRequest(t, req, http.StatusForbidden)
		})

		t.Run("ApproveAsWriterNonAdmin", func(t *testing.T) {
			doAPIAddCollaborator(user2APICtx, user4.Name, perm.AccessModeWrite)(t)

			// run2 is still blocked, so this exercises the approval itself rather than the idempotent path
			req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/%d/approve", baseRepo.FullName(), run2.ID)).
				AddTokenAuth(user4Token)
			resp := MakeRequest(t, req, http.StatusOK)
			apiRun := DecodeJSON(t, resp, &api.ActionWorkflowRun{})
			assert.Equal(t, run2.ID, apiRun.ID)
			assert.NotEqual(t, "waiting", apiRun.Status, "approved run should not be blocked")
			assertApproved(t, run2.ID, user4.ID)
		})
	})
}

// testAPIActionsApproveRunNotRequired covers the run that never awaited approval, which
// must not report success just because NeedApproval is already false.
func testAPIActionsApproveRunNotRequired(t *testing.T) {
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	run := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: 795})
	require.False(t, run.NeedApproval)
	require.Zero(t, run.ApprovedBy)

	req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/approve", repo.FullName())).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusConflict)
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

// seedTaskLogs writes log rows for a task the way the runner does, so the log download
// endpoints have something to serve. Logs are placed in DBFS to keep the test independent
// of the object storage fixture.
func seedTaskLogs(t *testing.T, taskID int64, lines ...string) {
	t.Helper()

	task, err := actions_model.GetTaskByID(t.Context(), taskID)
	require.NoError(t, err)

	task.LogInStorage = false
	task.LogFilename = fmt.Sprintf("test-logs/%d.log", task.ID)
	task.LogLength, task.LogSize, task.LogIndexes = 0, 0, nil

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

func TestAPIActionsGetWorkflowRunLogs(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	t.Run("Success", func(t *testing.T) {
		// run 795 (workflow "test.yaml") has job 198 "job_1" on task 53 and job 199 "job_2" on task 54
		seedTaskLogs(t, 53, "hello from job_1")
		seedTaskLogs(t, 54, "hello from job_2")

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/logs", repo.FullName())).
			AddTokenAuth(token)
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

	t.Run("NoLogsAfterRerun", func(t *testing.T) {
		// the rerun starts a new attempt whose jobs have no task yet, so it has no logs
		req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/rerun", repo.FullName())).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/runs/795/logs", repo.FullName())).
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

	t.Run("Success", func(t *testing.T) {
		seedTaskLogs(t, 53, "hello from job_1")

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/actions/jobs/198/logs", repo.FullName())).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, "text/plain; charset=utf-8", resp.Header().Get("Content-Type"))
		assert.Contains(t, resp.Header().Get("Content-Disposition"), "test-job_1-53.log")
		assert.Contains(t, resp.Body.String(), "hello from job_1")
	})

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
