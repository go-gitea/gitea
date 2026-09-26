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

	"github.com/stretchr/testify/assert"
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
