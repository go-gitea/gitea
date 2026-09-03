// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"strconv"
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsQueue(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	ctx := t.Context()

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}) // public, owned by user2

	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3}) // owned by org3, for the filters

	// A queued job is waiting and unclaimed, so it appears in its repo's Actions-tab queue and in the
	// instance-wide admin queue.
	insertQueuedJob := func(repo *repo_model.Repository, index int64, jobName string) *actions_model.ActionRunJob {
		run := &actions_model.ActionRun{
			Title:         "queue-test",
			RepoID:        repo.ID,
			OwnerID:       repo.OwnerID,
			Index:         index,
			WorkflowID:    "test.yaml",
			TriggerUserID: user2.ID,
			Ref:           "refs/heads/master",
			CommitSHA:     "c2d72f548424103f01ee1dc02889c1e2bff816b0",
			Event:         "push",
			TriggerEvent:  "push",
			EventPayload:  "{}",
			Status:        actions_model.StatusWaiting,
		}
		require.NoError(t, db.Insert(ctx, run))
		job := &actions_model.ActionRunJob{
			RunID:   run.ID,
			RepoID:  repo.ID,
			OwnerID: repo.OwnerID,
			Name:    jobName,
			JobID:   jobName,
			RunsOn:  []string{"ubuntu-latest"},
			Status:  actions_model.StatusWaiting,
		}
		require.NoError(t, db.Insert(ctx, job))
		return job
	}
	const queuedJobName, otherJobName = "queued-job-marker", "queued-job-other-owner"
	job := insertQueuedJob(repo1, 8801, queuedJobName)
	insertQueuedJob(repo3, 8802, otherJobName)

	// A reusable-workflow caller lives in the same run as its children and turns Running by aggregating their status,
	// but caller should not be in the running job list since it is never claimed by a runner.
	const callerJobName = "reusable-caller-marker"
	require.NoError(t, db.Insert(ctx, &actions_model.ActionRunJob{
		RunID:            job.RunID,
		RepoID:           repo1.ID,
		OwnerID:          repo1.OwnerID,
		Name:             callerJobName,
		JobID:            callerJobName,
		Status:           actions_model.StatusRunning,
		IsReusableCaller: true,
	}))

	sessionAdmin := loginUser(t, "user1") // site admin
	sessionUser2 := loginUser(t, user2.Name)
	sessionUser4 := loginUser(t, "user4") // unrelated user (repo1 is public, so may read but not reorder)

	const repoQueue = "/user2/repo1/actions/queue"

	// Repo Actions-tab queue: a repo admin sees the queued job, but the view is read-only.
	body := sessionUser2.MakeRequest(t, NewRequest(t, "GET", repoQueue), http.StatusOK).Body.String()
	assert.Contains(t, body, queuedJobName)
	assert.Contains(t, body, "actions-queue-tbody")
	assert.NotContains(t, body, "drag-handle", "the repo queue is read-only, reordering is site-admin only")
	assert.Contains(t, body, "actions-management", "queue sits under Management in the Actions sidebar")
	assert.NotContains(t, body, callerJobName, "a reusable caller occupies no runner, so it is not a running job")
	assert.Contains(t, body, `class="item flex-text-block silenced selected" href="/user2/repo1/actions/queue"`)

	// The Actions runs list exposes Queue under Management, not as a top tab.
	listBody := sessionUser2.MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/actions"), http.StatusOK).Body.String()
	assert.Contains(t, listBody, "actions-management")
	assert.Contains(t, listBody, `href="/user2/repo1/actions/queue"`)
	assert.NotContains(t, listBody, `class="item flex-text-block silenced selected" href="/user2/repo1/actions/queue"`, "Queue is not selected on the runs list")

	// A non-admin reader of the public repo may view the queue too.
	body4 := sessionUser4.MakeRequest(t, NewRequest(t, "GET", repoQueue), http.StatusOK).Body.String()
	assert.Contains(t, body4, queuedJobName)
	assert.NotContains(t, body4, "drag-handle")

	// The instance-wide admin queue lists the same job.
	const adminQueue = "/-/admin/actions/queue"
	adminBody := func(query string) string {
		return sessionAdmin.MakeRequest(t, NewRequest(t, "GET", adminQueue+query), http.StatusOK).Body.String()
	}
	unfiltered := adminBody("")
	assert.Contains(t, unfiltered, queuedJobName)
	assert.Contains(t, unfiltered, otherJobName)
	// The filter dropdowns only offer repositories that have pending work, so both are listed.
	assert.Contains(t, unfiltered, "repo_id="+strconv.FormatInt(repo1.ID, 10))
	assert.Contains(t, unfiltered, "repo_id="+strconv.FormatInt(repo3.ID, 10))

	// Filters narrow the merged list: by status, by owner and by repository.
	assert.NotContains(t, adminBody("?status=running"), queuedJobName, "a waiting job is hidden by the running filter")
	assert.NotContains(t, adminBody("?status=running"), callerJobName)
	assert.Contains(t, adminBody("?status=waiting"), queuedJobName)

	byOwner := adminBody("?owner_id=" + strconv.FormatInt(user2.ID, 10))
	assert.Contains(t, byOwner, queuedJobName)
	assert.NotContains(t, byOwner, otherJobName, "org3's job is not user2's")

	byRepo := adminBody("?repo_id=" + strconv.FormatInt(repo3.ID, 10))
	assert.Contains(t, byRepo, otherJobName)
	assert.NotContains(t, byRepo, queuedJobName, "repo1's job is not repo3's")
	// An owner/repository filter hides the reorder handles: dropped-row neighbours would not be the real ones.
	assert.NotContains(t, byRepo, "drag-handle")

	// The auto-refresh endpoint returns just the list fragment (no full-page chrome), still listing the job.
	refresh := sessionUser2.MakeRequest(t, NewRequest(t, "GET", repoQueue+"?refresh=1"), http.StatusOK).Body.String()
	assert.Contains(t, refresh, `id="actions-queue-list"`)
	assert.Contains(t, refresh, queuedJobName)
	assert.NotContains(t, refresh, `<html`, "the refresh response is a fragment, not a full page")

	// queue_rank orders the whole instance, so repos have no reorder endpoint at all: a repo admin could
	// otherwise push their jobs ahead of every other repository's.
	moveForm := map[string]string{"id": strconv.FormatInt(job.ID, 10)}
	sessionUser2.MakeRequest(t, NewRequestWithValues(t, "POST", repoQueue+"/move", moveForm), http.StatusNotFound)
	sessionUser4.MakeRequest(t, NewRequestWithValues(t, "POST", repoQueue+"/move", moveForm), http.StatusNotFound)
	assert.Zero(t, unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID}).QueueRank)

	sessionAdmin.MakeRequest(t, NewRequestWithValues(t, "POST", adminQueue+"/move", moveForm), http.StatusNoContent)

	// The admin move promoted the job: it now carries a negative queue rank.
	moved := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID})
	assert.Negative(t, moved.QueueRank)
}
