// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsJobQueue(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	ctx := t.Context()

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})

	insertQueuedJob := func(repo *repo_model.Repository, index int64, jobName string) *actions_model.ActionRunJob {
		run := &actions_model.ActionRun{RepoID: repo.ID, OwnerID: repo.OwnerID, Index: index, Status: actions_model.StatusWaiting}
		require.NoError(t, db.Insert(ctx, run))
		job := &actions_model.ActionRunJob{RunID: run.ID, RepoID: repo.ID, Name: jobName, Status: actions_model.StatusWaiting}
		require.NoError(t, db.Insert(ctx, job))
		return job
	}
	const queuedJobName, otherJobName, callerJobName = "queued-job-marker", "queued-job-other-owner", "reusable-caller-marker"
	job := insertQueuedJob(repo1, 8801, queuedJobName)
	insertQueuedJob(repo3, 8802, otherJobName)
	require.NoError(t, db.Insert(ctx, &actions_model.ActionRunJob{
		RunID:            job.RunID,
		RepoID:           repo1.ID,
		Name:             callerJobName,
		Status:           actions_model.StatusRunning,
		IsReusableCaller: true,
	}))

	const repoJobQueue = "/user2/repo1/actions/job_queue"
	sessionUser2 := loginUser(t, "user2")
	repoDoc := NewHTMLParser(t, sessionUser2.MakeRequest(t, NewRequest(t, "GET", repoJobQueue+"?workflow=test.yaml"), http.StatusOK).Body)
	assert.Contains(t, repoDoc.Find("#actions-job-queue tbody").Text(), queuedJobName)
	assert.NotContains(t, repoDoc.Find("#actions-job-queue tbody").Text(), callerJobName)
	assert.Equal(t, 1, repoDoc.Find(`.flex-container-nav a.active[href="`+repoJobQueue+`"]`).Length())
	assert.Zero(t, repoDoc.Find(`.flex-container-nav a.active:not([href="`+repoJobQueue+`"])`).Length())

	listDoc := NewHTMLParser(t, sessionUser2.MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/actions"), http.StatusOK).Body)
	assert.Equal(t, 1, listDoc.Find(`.flex-container-nav a:not(.active)[href="`+repoJobQueue+`"]`).Length())

	assert.Contains(t, MakeRequest(t, NewRequest(t, "GET", repoJobQueue), http.StatusOK).Body.String(), queuedJobName)

	sessionAdmin := loginUser(t, "user1")
	adminGet := func(link string) (string, *HTMLDoc) {
		body := sessionAdmin.MakeRequest(t, NewRequest(t, "GET", link), http.StatusOK).Body.String()
		return body, NewHTMLParser(t, strings.NewReader(body))
	}
	refreshLinkOf := func(doc *HTMLDoc) string {
		link, ok := doc.Find("#actions-job-queue").Attr("data-job-queue-refresh-link")
		require.True(t, ok)
		return link
	}
	repoFilterSelector := func(repoID int64) string {
		return `#actions-job-queue-filter a[href^="?repo_id=` + strconv.FormatInt(repoID, 10) + `&"]`
	}

	const adminJobQueue = "/-/admin/actions/job_queue"
	unfiltered, unfilteredDoc := adminGet(adminJobQueue)
	assert.Contains(t, unfiltered, queuedJobName)
	assert.Contains(t, unfiltered, otherJobName)
	assert.Equal(t, 1, unfilteredDoc.Find(repoFilterSelector(repo1.ID)).Length())

	refresh, refreshDoc := adminGet(refreshLinkOf(unfilteredDoc))
	assert.NotContains(t, refresh, "<html")
	assert.Contains(t, refresh, queuedJobName)
	assert.Equal(t, 1, refreshDoc.Find(repoFilterSelector(repo3.ID)).Length())

	running, _ := adminGet(adminJobQueue + "?status=running")
	assert.NotContains(t, running, queuedJobName)
	assert.NotContains(t, running, callerJobName)

	byOwner, _ := adminGet(adminJobQueue + "?owner_id=" + strconv.FormatInt(repo1.OwnerID, 10))
	assert.Contains(t, byOwner, queuedJobName)
	assert.NotContains(t, byOwner, otherJobName)

	byRepo, _ := adminGet(adminJobQueue + "?repo_id=" + strconv.FormatInt(repo3.ID, 10))
	assert.Contains(t, byRepo, otherJobName)
	assert.NotContains(t, byRepo, queuedJobName)

	for _, query := range []string{"?repo_id=987654321", "?owner_id=987654321"} {
		body, doc := adminGet(adminJobQueue + query)
		assert.Contains(t, body, queuedJobName)
		assert.Contains(t, body, otherJobName)
		assert.NotContains(t, refreshLinkOf(doc), "987654321")
	}

	_, err := db.GetEngine(ctx).Where("repo_id = ?", repo3.ID).Cols("status").Update(&actions_model.ActionRunJob{Status: actions_model.StatusSuccess})
	require.NoError(t, err)
	for _, scope := range []string{"repo_id=" + strconv.FormatInt(repo3.ID, 10), "owner_id=" + strconv.FormatInt(repo3.OwnerID, 10)} {
		body, doc := adminGet(adminJobQueue + "?" + scope)
		assert.NotContains(t, body, queuedJobName)
		assert.NotContains(t, body, otherJobName)
		assert.Contains(t, refreshLinkOf(doc), scope)
	}
}
