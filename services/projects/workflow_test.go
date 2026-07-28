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

// TestIssueChangeProjectColumnDefaultColumnAsSource ensures item_column_changed
// workflows fire when an issue is dragged out of a project's default/unassigned
// column. project_issue.project_board_id is stored as 0 for that column (see
// LoadProjectIssueColumnMap), so MoveIssuesOnProjectColumn passes oldColumnID=0
// here, and that must resolve to the real default column instead of being
// treated as "no column" (which used to make the workflow lookup fail silently).
func TestIssueChangeProjectColumnDefaultColumnAsSource(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// project_issue fixture id 2: issue 2 sits in project 1's default column
	// (project_board_id 0), project_board fixture id 1 is project 1's default column.
	require.NoError(t, project_model.CreateWorkflow(ctx, &project_model.Workflow{
		ProjectID:     1,
		WorkflowEvent: project_model.WorkflowEventItemColumnChanged,
		Enabled:       true,
		WorkflowFilters: []project_model.WorkflowFilter{
			{Type: project_model.WorkflowFilterTypeSourceColumn, Value: "1"},
		},
		WorkflowActions: []project_model.WorkflowAction{
			{Type: project_model.WorkflowActionTypeAddLabels, Value: "2"},
		},
	}))

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})

	// oldColumnID=0 mirrors what MoveIssuesOnProjectColumn passes when a card is
	// dragged out of the project's default/unassigned column, newColumnID=2 is
	// project 1's "In Progress" column.
	(&workflowNotifier{}).IssueChangeProjectColumn(ctx, nil, issue, 0, 2)

	hasLabel := false
	for _, label := range issue.Labels {
		if label.ID == 2 {
			hasLabel = true
			break
		}
	}
	assert.True(t, hasLabel, "item_column_changed workflow must fire when the source column is the project's default column")
}
