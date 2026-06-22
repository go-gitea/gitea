// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	unittest "gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	web_context "gitea.dev/services/context"

	act_model "gitea.com/gitea/runner/act/model"
	"github.com/stretchr/testify/assert"
)

func TestReadWorkflow_WorkflowDispatchConfig(t *testing.T) {
	yaml := `
    name: local-action-docker-url
    `
	workflow, err := act_model.ReadWorkflow(strings.NewReader(yaml))
	assert.NoError(t, err, "read workflow should succeed")
	workflowDispatch := workflowDispatchConfig(workflow)
	assert.Nil(t, workflowDispatch)

	yaml = `
    name: local-action-docker-url
    on: push
    `
	workflow, err = act_model.ReadWorkflow(strings.NewReader(yaml))
	assert.NoError(t, err, "read workflow should succeed")
	workflowDispatch = workflowDispatchConfig(workflow)
	assert.Nil(t, workflowDispatch)

	yaml = `
    name: local-action-docker-url
    on: workflow_dispatch
    `
	workflow, err = act_model.ReadWorkflow(strings.NewReader(yaml))
	assert.NoError(t, err, "read workflow should succeed")
	workflowDispatch = workflowDispatchConfig(workflow)
	assert.NotNil(t, workflowDispatch)
	assert.Nil(t, workflowDispatch.Inputs)

	yaml = `
    name: local-action-docker-url
    on: [push, pull_request]
    `
	workflow, err = act_model.ReadWorkflow(strings.NewReader(yaml))
	assert.NoError(t, err, "read workflow should succeed")
	workflowDispatch = workflowDispatchConfig(workflow)
	assert.Nil(t, workflowDispatch)

	yaml = `
    name: local-action-docker-url
    on:
        push:
        pull_request:
    `
	workflow, err = act_model.ReadWorkflow(strings.NewReader(yaml))
	assert.NoError(t, err, "read workflow should succeed")
	workflowDispatch = workflowDispatchConfig(workflow)
	assert.Nil(t, workflowDispatch)

	yaml = `
    name: local-action-docker-url
    on: [push, workflow_dispatch]
    `
	workflow, err = act_model.ReadWorkflow(strings.NewReader(yaml))
	assert.NoError(t, err, "read workflow should succeed")
	workflowDispatch = workflowDispatchConfig(workflow)
	assert.NotNil(t, workflowDispatch)
	assert.Nil(t, workflowDispatch.Inputs)

	yaml = `
    name: local-action-docker-url
    on:
        - push
        - workflow_dispatch
    `
	workflow, err = act_model.ReadWorkflow(strings.NewReader(yaml))
	assert.NoError(t, err, "read workflow should succeed")
	workflowDispatch = workflowDispatchConfig(workflow)
	assert.NotNil(t, workflowDispatch)
	assert.Nil(t, workflowDispatch.Inputs)

	yaml = `
    name: local-action-docker-url
    on:
        push:
        pull_request:
        workflow_dispatch:
            inputs:
    `
	workflow, err = act_model.ReadWorkflow(strings.NewReader(yaml))
	assert.NoError(t, err, "read workflow should succeed")
	workflowDispatch = workflowDispatchConfig(workflow)
	assert.NotNil(t, workflowDispatch)
	assert.Nil(t, workflowDispatch.Inputs)

	yaml = `
    name: local-action-docker-url
    on:
        push:
        pull_request:
        workflow_dispatch:
            inputs:
                logLevel:
                    description: 'Log level'
                    required: true
                    default: 'warning'
                    type: choice
                    options:
                    - info
                    - warning
                    - debug
                boolean_default_true:
                    description: 'Test scenario tags'
                    required: true
                    type: boolean
                    default: true
                boolean_default_false:
                    description: 'Test scenario tags'
                    required: true
                    type: boolean
                    default: false
    `

	workflow, err = act_model.ReadWorkflow(strings.NewReader(yaml))
	assert.NoError(t, err, "read workflow should succeed")
	workflowDispatch = workflowDispatchConfig(workflow)
	assert.NotNil(t, workflowDispatch)
	assert.Equal(t, WorkflowDispatchInput{
		Name:        "logLevel",
		Default:     "warning",
		Description: "Log level",
		Options: []string{
			"info",
			"warning",
			"debug",
		},
		Required: true,
		Type:     "choice",
	}, workflowDispatch.Inputs[0])
	assert.Equal(t, WorkflowDispatchInput{
		Name:        "boolean_default_true",
		Default:     "true",
		Description: "Test scenario tags",
		Required:    true,
		Type:        "boolean",
	}, workflowDispatch.Inputs[1])
	assert.Equal(t, WorkflowDispatchInput{
		Name:        "boolean_default_false",
		Default:     "false",
		Description: "Test scenario tags",
		Required:    true,
		Type:        "boolean",
	}, workflowDispatch.Inputs[2])
}

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
	defer test.MockVariableValue(&setting.IsInTesting, true)()
	defer test.MockVariableValue(&setting.AppURL, "https://gitea.example.com/")()
	defer test.MockVariableValue(&setting.AppSubURL, "")()
	defer test.MockVariableValue(&setting.PublicURLDetection, setting.PublicURLNever)()

	t.Run("no workflow selected", func(t *testing.T) {
		ctx := newWorkflowBadgeTestContext(t)

		prepareWorkflowBadgeTemplate(ctx, "", "ignored")

		assert.NotContains(t, ctx.Data, "WorkflowBadge")
	})

	t.Run("selected workflow", func(t *testing.T) {
		ctx := newWorkflowBadgeTestContext(t)

		prepareWorkflowBadgeTemplate(ctx, "build/test workflow.yml", `CI [prod]\build "fast" <ok>`)

		assert.Equal(t, workflowBadge{
			URL: "https://gitea.example.com/user1/repo1/actions/workflows/build/test%20workflow.yml/badge.svg?branch=release%2F1.0+%26+hotfix",
			Markdown: `[![CI \[prod\]\\build "fast" <ok>](https://gitea.example.com/user1/repo1/actions/workflows/build/test%20workflow.yml/badge.svg?branch=release%2F1.0+%26+hotfix)]` +
				`(https://gitea.example.com/user1/repo1/actions?workflow=build%2Ftest+workflow.yml)`,
			HTML: `<a href="https://gitea.example.com/user1/repo1/actions?workflow=build%2Ftest+workflow.yml"><img src="https://gitea.example.com/user1/repo1/actions/workflows/build/test%20workflow.yml/badge.svg?branch=release%2F1.0+%26+hotfix" alt="CI [prod]\build &#34;fast&#34; &lt;ok&gt;"></a>`,
		}, ctx.Data["WorkflowBadge"])
	})
}

func newWorkflowBadgeTestContext(t *testing.T) *web_context.Context {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "https://gitea.example.com/user1/repo1/actions", nil)
	resp := httptest.NewRecorder()
	ctx := web_context.NewWebContext(web_context.NewBaseContextForTest(resp, req), nil, nil)
	ctx.Repo.Repository = &repo_model.Repository{
		OwnerName:     "user1",
		Name:          "repo1",
		DefaultBranch: "release/1.0 & hotfix",
	}
	return ctx
}
