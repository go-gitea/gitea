// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"strings"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	unittest "code.gitea.io/gitea/models/unittest"

	act_model "github.com/nektos/act/pkg/model"
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

	runs, total, err := db.FindAndCount[actions_model.ActionRun](db.DefaultContext,
		actions_model.FindRunOptions{RepoID: 4, Ref: "refs/heads/test"})
	assert.NoError(t, err)
	assert.Len(t, runs, 1)
	assert.EqualValues(t, 1, total)
	for _, run := range runs {
		assert.False(t, run.IsRefDeleted)
	}

	assert.NoError(t, loadIsRefDeleted(db.DefaultContext, 4, runs))
	for _, run := range runs {
		assert.True(t, run.IsRefDeleted)
	}
}
