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

	// A queued job in repo1: waiting, unclaimed, so it appears in the repo's Actions-tab queue and the
	// instance-wide admin queue.
	run := &actions_model.ActionRun{
		Title:         "queue-test",
		RepoID:        repo1.ID,
		OwnerID:       user2.ID,
		Index:         8801,
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
	const queuedJobName = "queued-job-marker"
	job := &actions_model.ActionRunJob{
		RunID:   run.ID,
		RepoID:  repo1.ID,
		OwnerID: user2.ID,
		Name:    queuedJobName,
		JobID:   queuedJobName,
		RunsOn:  []string{"ubuntu-latest"},
		Status:  actions_model.StatusWaiting,
	}
	require.NoError(t, db.Insert(ctx, job))

	sessionAdmin := loginUser(t, "user1") // site admin
	sessionUser2 := loginUser(t, user2.Name)
	sessionUser4 := loginUser(t, "user4") // unrelated user (repo1 is public, so may read but not reorder)

	const repoQueue = "/user2/repo1/actions/queue"

	// Repo Actions-tab queue: a repo admin sees the queued job and the drag-to-reorder handles.
	body := sessionUser2.MakeRequest(t, NewRequest(t, "GET", repoQueue), http.StatusOK).Body.String()
	assert.Contains(t, body, queuedJobName)
	assert.Contains(t, body, "actions-queue-tbody")
	assert.Contains(t, body, "drag-handle", "repo admins get reorder handles")

	// A non-admin reader of the public repo may view the queue but gets no reorder handles.
	body4 := sessionUser4.MakeRequest(t, NewRequest(t, "GET", repoQueue), http.StatusOK).Body.String()
	assert.Contains(t, body4, queuedJobName)
	assert.NotContains(t, body4, "drag-handle", "non-admins get no reorder handles")

	// The instance-wide admin queue lists the same job.
	assert.Contains(t,
		sessionAdmin.MakeRequest(t, NewRequest(t, "GET", "/-/admin/actions/queue"), http.StatusOK).Body.String(),
		queuedJobName)

	// The auto-refresh endpoint returns just the list fragment (no full-page chrome), still listing the job.
	refresh := sessionUser2.MakeRequest(t, NewRequest(t, "GET", repoQueue+"?refresh=1"), http.StatusOK).Body.String()
	assert.Contains(t, refresh, `id="actions-queue-list"`)
	assert.Contains(t, refresh, queuedJobName)
	assert.NotContains(t, refresh, `<html`, "the refresh response is a fragment, not a full page")

	// Reordering is repo-admin only.
	moveForm := map[string]string{"id": strconv.FormatInt(job.ID, 10), "page": "1"}
	sessionUser4.MakeRequest(t, NewRequestWithValues(t, "POST", repoQueue+"/move", moveForm), http.StatusNotFound)
	sessionUser2.MakeRequest(t, NewRequestWithValues(t, "POST", repoQueue+"/move", moveForm), http.StatusNoContent)

	// The admin move promoted the job: it now carries a negative queue rank.
	moved := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID})
	assert.Negative(t, moved.QueueRank)
}
