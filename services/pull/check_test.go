// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"strconv"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/pull"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/services/automergequeue"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPullRequest_AddToTaskQueue(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	idChan := make(chan int64, 10)
	testHandler := func(items ...string) []string {
		for _, s := range items {
			id, _ := strconv.ParseInt(s, 10, 64)
			idChan <- id
		}
		return nil
	}

	cfg, err := setting.GetQueueSettings(setting.CfgProvider, "pr_patch_checker")
	assert.NoError(t, err)
	prPatchCheckerQueue, err = queue.NewWorkerPoolQueueWithContext(t.Context(), "pr_patch_checker", cfg, testHandler, true)
	assert.NoError(t, err)

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	StartPullRequestCheckImmediately(t.Context(), pr)

	assert.Eventually(t, func() bool {
		pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
		return pr.Status == issues_model.PullRequestStatusChecking
	}, 1*time.Second, 100*time.Millisecond)

	has, err := prPatchCheckerQueue.Has(strconv.FormatInt(pr.ID, 10))
	assert.True(t, has)
	assert.NoError(t, err)

	go prPatchCheckerQueue.Run()

	select {
	case id := <-idChan:
		assert.Equal(t, pr.ID, id)
	case <-time.After(time.Second):
		assert.FailNow(t, "Timeout: nothing was added to pullRequestQueue")
	}

	has, err = prPatchCheckerQueue.Has(strconv.FormatInt(pr.ID, 10))
	assert.False(t, has)
	assert.NoError(t, err)

	pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.Equal(t, issues_model.PullRequestStatusChecking, pr.Status)

	prPatchCheckerQueue.ShutdownWait(time.Second)
	prPatchCheckerQueue = nil
}

func TestMarkPullRequestAsMergeable(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	prPatchCheckerQueue = queue.CreateUniqueQueue(graceful.GetManager().ShutdownContext(), "pr_patch_checker", func(items ...string) []string { return nil })
	go prPatchCheckerQueue.Run()
	defer func() {
		prPatchCheckerQueue.ShutdownWait(time.Second)
		prPatchCheckerQueue = nil
	}()

	addToQueueShaChan := make(chan string, 1)
	defer test.MockVariableValue(&automergequeue.AddToQueue, func(pr *issues_model.PullRequest, sha string) {
		addToQueueShaChan <- sha
	})()
	ctx := t.Context()
	_, _ = db.GetEngine(ctx).ID(2).Update(&issues_model.PullRequest{Status: issues_model.PullRequestStatusChecking})
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	require.False(t, pr.HasMerged)
	require.Equal(t, issues_model.PullRequestStatusChecking, pr.Status)

	err := pull.ScheduleAutoMerge(ctx, &user_model.User{ID: 99999}, pr.ID, repo_model.MergeStyleMerge, "test msg", true)
	require.NoError(t, err)

	exist, scheduleMerge, err := pull.GetScheduledMergeByPullID(ctx, pr.ID)
	require.NoError(t, err)
	assert.True(t, exist)
	assert.True(t, scheduleMerge.Doer.IsGhost())

	markPullRequestAsMergeable(ctx, pr)
	pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	require.Equal(t, issues_model.PullRequestStatusMergeable, pr.Status)

	select {
	case sha := <-addToQueueShaChan:
		assert.Equal(t, "985f0301dba5e7b34be866819cd15ad3d8f508ee", sha) // ref: refs/pull/3/head
	case <-time.After(1 * time.Second):
		assert.FailNow(t, "Timeout: nothing was added to automergequeue")
	}
}
