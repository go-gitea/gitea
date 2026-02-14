// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertToViewModel_StepSummary(t *testing.T) {
	ctx, _ := contexttest.MockContext(t, "/")

	task := &actions_model.ActionTask{
		Status: actions_model.StatusSuccess,
		Steps: []*actions_model.ActionTaskStep{
			{Name: "actions/checkout@v4", Index: 0, Status: actions_model.StatusSuccess, LogLength: 1, Stopped: timeutil.TimeStamp(1)},
			{Name: "make build", Index: 1, Status: actions_model.StatusSuccess, LogLength: 1, Stopped: timeutil.TimeStamp(2)},
			{Name: "Run tests", Index: 2, Status: actions_model.StatusSuccess, LogLength: 1, Stopped: timeutil.TimeStamp(3)},
			{Name: "echo done", Index: 3, Status: actions_model.StatusSuccess, LogLength: 1, Stopped: timeutil.TimeStamp(4)},
		},
		Job: &actions_model.ActionRunJob{
			WorkflowPayload: []byte(`
name: test
on: push
jobs:
  job1:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: make build
      - name: Run tests
        run: make test
      - run: echo done
`),
		},
	}

	viewJobs, _, err := convertToViewModel(ctx, nil, task)
	require.NoError(t, err)

	var summaries []string
	for _, v := range viewJobs {
		summaries = append(summaries, v.Summary)
	}
	assert.Equal(t, []string{
		"Set up job",
		"actions.runs.run:actions/checkout@v4", // uses: without name gets prefix
		"actions.runs.run:make build",          // run: without name gets prefix
		"Run tests",                            // run: with name unchanged
		"actions.runs.run:echo done",           // run: without name gets prefix
		"Complete job",
	}, summaries)
}
