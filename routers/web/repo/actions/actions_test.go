// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestActionRunListData(t *testing.T) {
	unittest.PrepareTestEnv(t)
	t.Run("preparePartialRefreshRuns", func(t *testing.T) {
		ctx, _ := contexttest.MockContext(t, "user5/repo4/actions")
		contexttest.LoadRepo(t, ctx, 4)
		d := &actionRunListData{refreshRunIDs: []int64{791, 792}}
		d.preparePartialRefreshRuns(ctx)
		assert.Equal(t, []int64{791, 792}, []int64{d.ActionRuns[0].ID, d.ActionRuns[1].ID})
		d = &actionRunListData{refreshRunIDs: []int64{792, 791}}
		d.preparePartialRefreshRuns(ctx)
		assert.Equal(t, []int64{792, 791}, []int64{d.ActionRuns[0].ID, d.ActionRuns[1].ID})
	})
}

func TestActionRunListSorting(t *testing.T) {
	unittest.PrepareTestEnv(t)
	defer test.MockVariableValue(&setting.AppSubURL, "/gitea")()
	for _, sortType := range []string{"", "newest", "oldest", "status", "reverse_status", "ref", "reverse_ref", "invalid"} {
		t.Run(sortType, func(t *testing.T) {
			ctx, _ := contexttest.MockContext(t, "/user5/repo4/actions?workflow=artifact.yaml&actor=1&status=1&branch=master&limit=1&sort="+url.QueryEscape(sortType))
			contexttest.LoadRepo(t, ctx, 4)
			ctx.Req.RequestURI = ctx.Req.URL.RequestURI()
			data := prepareWorkflowList(ctx, "artifact.yaml")
			expected := sortType
			if sortType == "invalid" {
				expected = ""
			}
			assert.Equal(t, expected, data.sort)
			if !assert.True(t, data.prepareFullPageRuns(ctx, nil)) {
				return
			}
			assert.Len(t, data.ActionRuns, 1)
			if !assert.True(t, data.prepareFullPageTemplate(ctx)) {
				return
			}
			assert.Equal(t, expected, ctx.Data["SortType"])
			assert.Contains(t, ctx.Data["RunSortLink"], "/gitea/user5/repo4/actions?")
			params, err := url.ParseQuery(string(data.pager.GetParams()))
			assert.NoError(t, err)
			assert.Equal(t, sortType, params.Get("sort"))
			assert.Equal(t, "master", params.Get("branch"))
			assert.Equal(t, "artifact.yaml", params.Get("workflow"))
			assert.Equal(t, "1", params.Get("actor"))
			assert.Equal(t, "1", params.Get("status"))
		})
	}
}
