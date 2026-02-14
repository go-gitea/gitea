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

	// Step names simulate what CreateTaskForRunner stores in the DB:
	// unnamed steps already have the "Run " prefix applied at creation time.
	task := &actions_model.ActionTask{
		Status: actions_model.StatusSuccess,
		Steps: []*actions_model.ActionTaskStep{
			{Name: "Run actions/checkout@v4", Index: 0, Status: actions_model.StatusSuccess, LogLength: 1, Stopped: timeutil.TimeStamp(1)},
			{Name: "Run make build", Index: 1, Status: actions_model.StatusSuccess, LogLength: 1, Stopped: timeutil.TimeStamp(2)},
			{Name: "Run tests", Index: 2, Status: actions_model.StatusSuccess, LogLength: 1, Stopped: timeutil.TimeStamp(3)},
			{Name: "Run echo done", Index: 3, Status: actions_model.StatusSuccess, LogLength: 1, Stopped: timeutil.TimeStamp(4)},
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
		"Run actions/checkout@v4", // uses: without name gets "Run " prefix from DB
		"Run make build",          // run: without name gets "Run " prefix from DB
		"Run tests",               // run: with explicit name, stored as-is
		"Run echo done",           // run: without name gets "Run " prefix from DB
		"Complete job",
	}, summaries)
}
