// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	actions_web "code.gitea.io/gitea/routers/web/repo/actions"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsRoute(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		t.Run("testActionsRouteForIDBasedURL", testActionsRouteForIDBasedURL)
		t.Run("testActionsRouteForLegacyIndexBasedURL", testActionsRouteForLegacyIndexBasedURL)
	})
}

func testActionsRouteForIDBasedURL(t *testing.T) {
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user2Session := loginUser(t, user2.Name)
	user2Token := getTokenForLoggedInUser(t, user2Session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

	repo1 := createActionsTestRepo(t, user2Token, "actions-route-id-url-1", false)
	runner1 := newMockRunner()
	runner1.registerAsRepoRunner(t, user2.Name, repo1.Name, "mock-runner", []string{"ubuntu-latest"}, false)
	repo2 := createActionsTestRepo(t, user2Token, "actions-route-id-url-2", false)
	runner2 := newMockRunner()
	runner2.registerAsRepoRunner(t, user2.Name, repo2.Name, "mock-runner", []string{"ubuntu-latest"}, false)

	workflowTreePath := ".gitea/workflows/test.yml"
	workflowContent := `name: test
on:
  push:
    paths:
      - '.gitea/workflows/test.yml'
jobs:
  job1:
    runs-on: ubuntu-latest
    steps:
      - run: echo job1
`

	opts := getWorkflowCreateFileOptions(user2, repo1.DefaultBranch, "create "+workflowTreePath, workflowContent)
	createWorkflowFile(t, user2Token, user2.Name, repo1.Name, workflowTreePath, opts)
	createWorkflowFile(t, user2Token, user2.Name, repo2.Name, workflowTreePath, opts)

	task1 := runner1.fetchTask(t)
	_, job1, run1 := getTaskAndJobAndRunByTaskID(t, task1.Id)
	task2 := runner2.fetchTask(t)
	_, job2, run2 := getTaskAndJobAndRunByTaskID(t, task2.Id)

	req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d", user2.Name, repo1.Name, run1.ID))
	user2Session.MakeRequest(t, req, http.StatusOK)

	req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d", user2.Name, repo1.Name, 999999))
	user2Session.MakeRequest(t, req, http.StatusNotFound)

	// run1 and job1 belong to repo1, success
	req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d", user2.Name, repo1.Name, run1.ID, job1.ID))
	resp := user2Session.MakeRequest(t, req, http.StatusOK)
	var viewResp actions_web.ViewResponse
	DecodeJSON(t, resp, &viewResp)
	assert.Len(t, viewResp.State.Run.Jobs, 1)
	assert.Equal(t, job1.ID, viewResp.State.Run.Jobs[0].ID)

	// run2 and job2 do not belong to repo1, failure
	req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d", user2.Name, repo1.Name, run2.ID, job2.ID))
	user2Session.MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d", user2.Name, repo1.Name, run1.ID, job2.ID))
	user2Session.MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d", user2.Name, repo1.Name, run2.ID, job1.ID))
	user2Session.MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d/workflow", user2.Name, repo1.Name, run2.ID))
	user2Session.MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/approve", user2.Name, repo1.Name, run2.ID))
	user2Session.MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/cancel", user2.Name, repo1.Name, run2.ID))
	user2Session.MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/delete", user2.Name, repo1.Name, run2.ID))
	user2Session.MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d/artifacts/test.txt", user2.Name, repo1.Name, run2.ID))
	user2Session.MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "DELETE", fmt.Sprintf("/%s/%s/actions/runs/%d/artifacts/test.txt", user2.Name, repo1.Name, run2.ID))
	user2Session.MakeRequest(t, req, http.StatusNotFound)

	// make the tasks complete, then test rerun
	runner1.execTask(t, task1, &mockTaskOutcome{
		result: runnerv1.Result_RESULT_SUCCESS,
	})
	runner2.execTask(t, task2, &mockTaskOutcome{
		result: runnerv1.Result_RESULT_SUCCESS,
	})
	req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/rerun", user2.Name, repo1.Name, run2.ID))
	user2Session.MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d/rerun", user2.Name, repo1.Name, run2.ID, job2.ID))
	user2Session.MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d/rerun", user2.Name, repo1.Name, run1.ID, job2.ID))
	user2Session.MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d/rerun", user2.Name, repo1.Name, run2.ID, job1.ID))
	user2Session.MakeRequest(t, req, http.StatusNotFound)
}

func testActionsRouteForLegacyIndexBasedURL(t *testing.T) {
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user2Session := loginUser(t, user2.Name)
	user2Token := getTokenForLoggedInUser(t, user2Session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
	repo := createActionsTestRepo(t, user2Token, "actions-route-legacy-url", false)

	mkRun := func(id, index int64, title, sha string) *actions_model.ActionRun {
		return &actions_model.ActionRun{
			ID:            id,
			Index:         index,
			RepoID:        repo.ID,
			OwnerID:       user2.ID,
			Title:         title,
			WorkflowID:    "legacy-route.yml",
			TriggerUserID: user2.ID,
			Ref:           "refs/heads/master",
			CommitSHA:     sha,
			Status:        actions_model.StatusWaiting,
		}
	}
	mkJob := func(id, runID int64, name, sha string) *actions_model.ActionRunJob {
		return &actions_model.ActionRunJob{
			ID:        id,
			RunID:     runID,
			RepoID:    repo.ID,
			OwnerID:   user2.ID,
			CommitSHA: sha,
			Name:      name,
			Status:    actions_model.StatusWaiting,
		}
	}

	// A small ID-based run/job pair that should always resolve directly.
	smallIDRun := mkRun(80, 20, "legacy route small id", "aaa001")
	smallIDJob := mkJob(170, smallIDRun.ID, "legacy-small-job", smallIDRun.CommitSHA)
	// Another small run used to provide a job ID that belongs to a different run.
	otherSmallRun := mkRun(90, 30, "legacy route other small", "aaa002")
	otherSmallJob := mkJob(180, otherSmallRun.ID, "legacy-other-small-job", otherSmallRun.CommitSHA)

	// A large-ID run whose legacy run index should redirect to its ID-based URL.
	normalRun := mkRun(1500, 900, "legacy route normal", "aaa003")
	normalRunJob := mkJob(1600, normalRun.ID, "legacy-normal-job", normalRun.CommitSHA)
	// A run whose index collides with normalRun.ID to exercise summary-page ID-first behavior.
	collisionRun := mkRun(2400, 1500, "legacy route collision", "aaa004")
	collisionJobIdx0 := mkJob(2600, collisionRun.ID, "legacy-collision-job-1", collisionRun.CommitSHA)
	collisionJobIdx1 := mkJob(2601, collisionRun.ID, "legacy-collision-job-2", collisionRun.CommitSHA)

	// A small ID-based run/job pair that collides with a different legacy run/job index pair.
	ambiguousIDRun := mkRun(3, 1, "legacy route ambiguous id", "aaa005")
	ambiguousIDJob := mkJob(4, ambiguousIDRun.ID, "legacy-ambiguous-id-job", ambiguousIDRun.CommitSHA)
	// The legacy run/job target for the ambiguous /runs/3/jobs/4 URL.
	ambiguousLegacyRun := mkRun(1501, ambiguousIDRun.ID, "legacy route ambiguous legacy", "aaa006")
	ambiguousLegacyJobIdx0 := mkJob(1601, ambiguousLegacyRun.ID, "legacy-ambiguous-legacy-job-0", ambiguousLegacyRun.CommitSHA)
	ambiguousLegacyJobIdx1 := mkJob(1602, ambiguousLegacyRun.ID, "legacy-ambiguous-legacy-job-1", ambiguousLegacyRun.CommitSHA)
	ambiguousLegacyJobIdx2 := mkJob(1603, ambiguousLegacyRun.ID, "legacy-ambiguous-legacy-job-2", ambiguousLegacyRun.CommitSHA)
	ambiguousLegacyJobIdx3 := mkJob(1604, ambiguousLegacyRun.ID, "legacy-ambiguous-legacy-job-3", ambiguousLegacyRun.CommitSHA)
	ambiguousLegacyJobIdx4 := mkJob(1605, ambiguousLegacyRun.ID, "legacy-ambiguous-legacy-job-4", ambiguousLegacyRun.CommitSHA) // job_index=4
	ambiguousLegacyJobIdx5 := mkJob(1606, ambiguousLegacyRun.ID, "legacy-ambiguous-legacy-job-5", ambiguousLegacyRun.CommitSHA)
	ambiguousLegacyJobs := []*actions_model.ActionRunJob{
		ambiguousLegacyJobIdx0,
		ambiguousLegacyJobIdx1,
		ambiguousLegacyJobIdx2,
		ambiguousLegacyJobIdx3,
		ambiguousLegacyJobIdx4,
		ambiguousLegacyJobIdx5,
	}
	targetAmbiguousLegacyJob := ambiguousLegacyJobs[int(ambiguousIDJob.ID)]

	insertBeansWithExplicitIDs(t, "action_run",
		smallIDRun, otherSmallRun, normalRun, ambiguousIDRun, ambiguousLegacyRun, collisionRun,
	)
	insertBeansWithExplicitIDs(t, "action_run_job",
		smallIDJob, otherSmallJob, normalRunJob, ambiguousIDJob, collisionJobIdx0, collisionJobIdx1,
		ambiguousLegacyJobIdx0, ambiguousLegacyJobIdx1, ambiguousLegacyJobIdx2, ambiguousLegacyJobIdx3, ambiguousLegacyJobIdx4, ambiguousLegacyJobIdx5,
	)

	t.Run("OnlyRunID", func(t *testing.T) {
		// ID-based URLs must be valid
		req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d", user2.Name, repo.Name, smallIDRun.ID))
		user2Session.MakeRequest(t, req, http.StatusOK)
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d", user2.Name, repo.Name, normalRun.ID))
		user2Session.MakeRequest(t, req, http.StatusOK)
	})

	t.Run("OnlyRunIndex", func(t *testing.T) {
		// legacy run index should redirect to the ID-based URL
		req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d", user2.Name, repo.Name, normalRun.Index))
		resp := user2Session.MakeRequest(t, req, http.StatusFound)
		assert.Equal(t, fmt.Sprintf("/%s/%s/actions/runs/%d", user2.Name, repo.Name, normalRun.ID), resp.Header().Get("Location"))

		// Best-effort compatibility prefers the run ID when the same number also exists as a legacy run index.
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d", user2.Name, repo.Name, collisionRun.Index))
		resp = user2Session.MakeRequest(t, req, http.StatusOK)
		assert.Contains(t, resp.Body.String(), fmt.Sprintf(`data-run-id="%d"`, normalRun.ID)) // because collisionRun.Index == normalRun.ID

		// by_index=1 should force the summary page to use the legacy run index interpretation.
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d?by_index=1", user2.Name, repo.Name, collisionRun.Index))
		resp = user2Session.MakeRequest(t, req, http.StatusFound)
		assert.Equal(t, fmt.Sprintf("/%s/%s/actions/runs/%d", user2.Name, repo.Name, collisionRun.ID), resp.Header().Get("Location"))
	})

	t.Run("RunIDAndJobID", func(t *testing.T) {
		// ID-based URLs must be valid
		req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d", user2.Name, repo.Name, smallIDRun.ID, smallIDJob.ID))
		user2Session.MakeRequest(t, req, http.StatusOK)
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d", user2.Name, repo.Name, normalRun.ID, normalRunJob.ID))
		user2Session.MakeRequest(t, req, http.StatusOK)
	})

	t.Run("RunIndexAndJobIndex", func(t *testing.T) {
		// /user2/repo2/actions/runs/3/jobs/4 is ambiguous:
		//   - it may resolve as the ID-based URL for run_id=3/job_id=4,
		//   - or as the legacy index-based URL for run_index=3/job_index=4 which should redirect to run_id=1501/job_id=1605.
		idBasedURL := fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d", user2.Name, repo.Name, ambiguousIDRun.ID, ambiguousIDJob.ID)
		indexBasedURL := fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d", user2.Name, repo.Name, ambiguousLegacyRun.Index, 4) // for ambiguousLegacyJobIdx4
		assert.Equal(t, idBasedURL, indexBasedURL)
		// When both interpretations are valid, prefer the ID-based target by default.
		req := NewRequest(t, "GET", indexBasedURL)
		user2Session.MakeRequest(t, req, http.StatusOK)
		redirectURL := fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d", user2.Name, repo.Name, ambiguousLegacyRun.ID, targetAmbiguousLegacyJob.ID)
		// by_index=1 should explicitly force the legacy run/job index interpretation.
		req = NewRequest(t, "GET", indexBasedURL+"?by_index=1")
		resp := user2Session.MakeRequest(t, req, http.StatusFound)
		assert.Equal(t, redirectURL, resp.Header().Get("Location"))

		// legacy job index 0 should redirect to the first job's ID
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/0", user2.Name, repo.Name, collisionRun.Index))
		resp = user2Session.MakeRequest(t, req, http.StatusFound)
		assert.Equal(t, fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d", user2.Name, repo.Name, collisionRun.ID, collisionJobIdx0.ID), resp.Header().Get("Location"))

		// legacy job index 1 should redirect to the second job's ID
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/1", user2.Name, repo.Name, collisionRun.Index))
		resp = user2Session.MakeRequest(t, req, http.StatusFound)
		assert.Equal(t, fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d", user2.Name, repo.Name, collisionRun.ID, collisionJobIdx1.ID), resp.Header().Get("Location"))
	})

	t.Run("InvalidURLs", func(t *testing.T) {
		// the job ID from a different run should not match
		req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/%d", user2.Name, repo.Name, smallIDRun.ID, otherSmallJob.ID))
		user2Session.MakeRequest(t, req, http.StatusNotFound)

		// resolve the run by index first and then return not found because the job index is out-of-range
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/2", user2.Name, repo.Name, normalRun.ID))
		user2Session.MakeRequest(t, req, http.StatusNotFound)

		// an out-of-range job index should return not found
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/2", user2.Name, repo.Name, collisionRun.Index))
		user2Session.MakeRequest(t, req, http.StatusNotFound)

		// a missing run number should return not found
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d", user2.Name, repo.Name, 999999))
		user2Session.MakeRequest(t, req, http.StatusNotFound)

		// a missing legacy run index should return not found
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/actions/runs/%d/jobs/0", user2.Name, repo.Name, 999999))
		user2Session.MakeRequest(t, req, http.StatusNotFound)
	})
}

func insertBeansWithExplicitIDs(t *testing.T, table string, beans ...any) {
	t.Helper()
	ctx, committer, err := db.TxContext(t.Context())
	require.NoError(t, err)
	defer committer.Close()

	if setting.Database.Type.IsMSSQL() {
		_, err = db.Exec(ctx, fmt.Sprintf("SET IDENTITY_INSERT [%s] ON", table))
		require.NoError(t, err)
		defer func() {
			_, err = db.Exec(ctx, fmt.Sprintf("SET IDENTITY_INSERT [%s] OFF", table))
			require.NoError(t, err)
		}()
	}
	require.NoError(t, db.Insert(ctx, beans...))
	require.NoError(t, committer.Commit())
}
