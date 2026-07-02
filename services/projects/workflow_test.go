// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package projects

import (
	"context"
	"strconv"
	"testing"

	issues_model "gitea.dev/models/issues"
	project_model "gitea.dev/models/project"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/translation"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetWorkflowSummary(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	column := unittest.AssertExistsAndLoadBean(t, &project_model.Column{ID: 1})
	label := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 1})

	ctx := context.WithValue(t.Context(), translation.ContextKey, translation.MockLocale{})
	workflow := &project_model.Workflow{
		WorkflowFilters: []project_model.WorkflowFilter{
			{Type: project_model.WorkflowFilterTypeIssueType, Value: "issue"},
			{Type: project_model.WorkflowFilterTypeSourceColumn, Value: strconv.FormatInt(column.ID, 10)},
			{Type: project_model.WorkflowFilterTypeTargetColumn, Value: strconv.FormatInt(column.ID, 10)},
			{Type: project_model.WorkflowFilterTypeLabels, Value: strconv.FormatInt(label.ID, 10)},
		},
	}

	assert.Equal(t,
		"(projects.workflows.issues_only) "+
			"(projects.workflows.summary.source:"+column.Title+") "+
			"(projects.workflows.summary.target:"+column.Title+") "+
			"(projects.workflows.summary.labels:"+label.Name+")",
		GetWorkflowSummary(ctx, workflow),
	)
}
