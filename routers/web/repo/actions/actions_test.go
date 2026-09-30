// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"net/http"
	"net/http/httptest"
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	unittest "gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	web_context "gitea.dev/services/context"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_loadIsRefDeleted(t *testing.T) {
	unittest.PrepareTestEnv(t)

	runs, total, err := db.FindAndCount[actions_model.ActionRun](t.Context(),
		actions_model.FindRunOptions{RepoID: 4, Ref: "refs/heads/test"})
	assert.NoError(t, err)
	assert.Len(t, runs, 1)
	assert.EqualValues(t, 1, total)
	for _, run := range runs {
		assert.False(t, run.IsRefDeleted)
	}

	assert.NoError(t, loadIsRefDeleted(t.Context(), 4, runs))
	for _, run := range runs {
		assert.True(t, run.IsRefDeleted)
	}
}

func TestPrepareWorkflowBadgeTemplate(t *testing.T) {
	defer test.MockVariableValue(&setting.AppURL, "https://gitea.example.com/")()

	t.Run("no workflow selected", func(t *testing.T) {
		ctx := newWorkflowBadgeTestContext(t)

		prepareWorkflowBadgeTemplate(ctx, "", "ignored")

		assert.NotContains(t, ctx.Data, "WorkflowBadge")
	})

	t.Run("selected workflow", func(t *testing.T) {
		ctx := newWorkflowBadgeTestContext(t)

		prepareWorkflowBadgeTemplate(ctx, "build/test workflow.yml", `CI [prod]\build "fast" <ok>`)

		assert.Equal(t, workflowBadge{
			BadgeURL:    "https://gitea.example.com/user1/repo1/actions/workflows/build/test%20workflow.yml/badge.svg?branch=release%2F1.0+%26+hotfix",
			WorkflowURL: "https://gitea.example.com/user1/repo1/actions?workflow=build%2Ftest+workflow.yml",
			DisplayName: `CI [prod]\build "fast" <ok>`,
		}, ctx.Data["WorkflowBadge"])
	})
}

func newWorkflowBadgeTestContext(t *testing.T) *web_context.Context {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "https://gitea.example.com/user1/repo1/actions", nil)
	resp := httptest.NewRecorder()
	ctx := web_context.NewWebContext(web_context.NewBaseContextForTest(t, resp, req), nil, nil)
	ctx.Repo.Repository = &repo_model.Repository{
		OwnerName:     "user1",
		Name:          "repo1",
		DefaultBranch: "release/1.0 & hotfix",
	}
	return ctx
}

func TestPreparePartialRefreshRunsKeepsRequestedOrder(t *testing.T) {
	unittest.PrepareTestEnv(t)

	refresh := func(t *testing.T, runIDs []int64) []int64 {
		ctx, _ := contexttest.MockContext(t, "user5/repo4/actions")
		contexttest.LoadRepo(t, ctx, 4)
		data := &actionRunListData{refreshRunIDs: runIDs}
		require.True(t, data.preparePartialRefreshRuns(ctx))
		ids := make([]int64, 0, len(data.ActionRuns))
		for _, run := range data.ActionRuns {
			ids = append(ids, run.ID)
		}
		return ids
	}

	t.Run("newest first, as the runs list renders them", func(t *testing.T) {
		assert.Equal(t, []int64{794, 793, 792, 791}, refresh(t, []int64{794, 793, 792, 791}))
	})

	t.Run("runs of other repositories are dropped", func(t *testing.T) {
		// run 795 belongs to repo 2
		assert.Equal(t, []int64{794, 791}, refresh(t, []int64{794, 795, 791}))
	})
}
