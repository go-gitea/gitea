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
	user_model "gitea.dev/models/user"
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
	triggeringUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// oldColumnID=0 mirrors what MoveIssuesOnProjectColumn passes when a card is
	// dragged out of the project's default/unassigned column, newColumnID=2 is
	// project 1's "In Progress" column. The doer is a real user (whoever moved
	// the card), never nil in production - MoveIssuesOnProjectColumn/
	// MoveIssueToAnotherColumn only ever run for an authenticated actor.
	(&workflowNotifier{}).IssueChangeProjectColumn(ctx, triggeringUser, issue, 0, 2)

	hasLabel := false
	for _, label := range issue.Labels {
		if label.ID == 2 {
			hasLabel = true
			break
		}
	}
	assert.True(t, hasLabel, "item_column_changed workflow must fire when the source column is the project's default column")
}

// TestIssueChangeProjectColumnNoOpWithinDefaultColumn ensures item_column_changed
// workflows do NOT fire for a no-op move within the project's default/unassigned
// column. Callers (MoveIssuesOnProjectColumn, MoveIssueToAnotherColumn) pass the
// raw project_issue.project_board_id, which is 0 for issues in the default column,
// so a reorder/drop that leaves the issue there arrives as oldColumnID=0 with
// newColumnID equal to the default column's real ID. Once oldColumnID=0 is
// resolved to that same real ID, this must be treated as no move at all.
func TestIssueChangeProjectColumnNoOpWithinDefaultColumn(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// project_issue fixture id 2: issue 2 sits in project 1's default column
	// (project_board_id 0), project_board fixture id 1 is project 1's default column.
	require.NoError(t, project_model.CreateWorkflow(ctx, &project_model.Workflow{
		ProjectID:     1,
		WorkflowEvent: project_model.WorkflowEventItemColumnChanged,
		Enabled:       true,
		WorkflowActions: []project_model.WorkflowAction{
			{Type: project_model.WorkflowActionTypeAddLabels, Value: "2"},
		},
	}))

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	triggeringUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// oldColumnID=0 resolves to column 1 (project 1's default column), and
	// newColumnID=1 is that same column, so this must be a no-op.
	(&workflowNotifier{}).IssueChangeProjectColumn(ctx, triggeringUser, issue, 0, 1)

	require.NoError(t, issue.LoadLabels(ctx))
	for _, label := range issue.Labels {
		assert.NotEqual(t, int64(2), label.ID, "item_column_changed workflow must not fire for a no-op move within the default column")
	}
}

// TestIssueChangeProjectColumnSkipsWorkflowDoer confirms the recursion guard
// (IsProjectWorkflowDoer) still works after the doer stopped being a synthetic
// user: NewProjectWorkflowDoer now wraps a real triggering user, so the guard
// must keep working via the ExtDoerData type assertion alone, regardless of
// whose real ID/Name the wrapped doer carries.
func TestIssueChangeProjectColumnSkipsWorkflowDoer(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	require.NoError(t, project_model.CreateWorkflow(ctx, &project_model.Workflow{
		ProjectID:     1,
		WorkflowEvent: project_model.WorkflowEventItemColumnChanged,
		Enabled:       true,
		WorkflowActions: []project_model.WorkflowAction{
			{Type: project_model.WorkflowActionTypeAddLabels, Value: "2"},
		},
	}))

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	realUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	workflowDoer := issues_model.NewProjectWorkflowDoer(realUser, "Some Project", 1, project_model.WorkflowEventItemColumnChanged)
	require.True(t, issues_model.IsProjectWorkflowDoer(workflowDoer))

	// Same move as TestIssueChangeProjectColumnDefaultColumnAsSource (which does
	// fire the workflow for a real, non-workflow doer), but here the doer is
	// itself a workflow doer, so this must be a no-op to prevent cascade loops.
	(&workflowNotifier{}).IssueChangeProjectColumn(ctx, workflowDoer, issue, 0, 2)

	require.NoError(t, issue.LoadLabels(ctx))
	for _, label := range issue.Labels {
		assert.NotEqual(t, int64(2), label.ID, "a workflow doer must never trigger another workflow")
	}
}

// TestExecuteWorkflowActionsNilTriggeringUser confirms that a nil triggeringUser
// (a caller bug in this file - every real notifier method resolves a real
// triggering user before reaching here) skips the workflow's actions entirely,
// rather than crashing (a bare nil-pointer dereference used to happen inside
// NewProjectWorkflowDoer/CreateComment) or half-applying some actions.
func TestExecuteWorkflowActionsNilTriggeringUser(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	workflow := &project_model.Workflow{
		ProjectID:     1,
		WorkflowEvent: project_model.WorkflowEventItemOpened,
		Enabled:       true,
		WorkflowActions: []project_model.WorkflowAction{
			{Type: project_model.WorkflowActionTypeAddLabels, Value: "2"},
		},
	}
	require.NoError(t, project_model.CreateWorkflow(ctx, workflow))

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})

	assert.NotPanics(t, func() {
		executeWorkflowActions(ctx, workflow, issue, nil, 1)
	})

	require.NoError(t, issue.LoadLabels(ctx))
	for _, label := range issue.Labels {
		assert.NotEqual(t, int64(2), label.ID, "a nil triggering user must skip the workflow's actions entirely")
	}
}

// TestMatchWorkflowsFilters pins the filter matching semantics. matchWorkflowsFilters
// is pure -- no context, no database -- so this needs no fixtures and covers every
// filter type in both directions, plus the fail-closed paths that keep a restrictive
// filter from degrading into "match everything".
func TestMatchWorkflowsFilters(t *testing.T) {
	const (
		sourceColumnID int64 = 10
		targetColumnID int64 = 20
		labelID        int64 = 30
	)

	issue := &issues_model.Issue{
		IsPull: false,
		Labels: []*issues_model.Label{{ID: labelID}},
	}
	pull := &issues_model.Issue{
		IsPull: true,
		Labels: []*issues_model.Label{{ID: labelID}},
	}
	unlabelled := &issues_model.Issue{IsPull: false}

	// item_column_changed is the only event whose capabilities include the source and
	// target column filters, so column cases must use it.
	const columnChanged = project_model.WorkflowEventItemColumnChanged
	const itemOpened = project_model.WorkflowEventItemOpened

	cases := []struct {
		name     string
		event    project_model.WorkflowEvent
		filters  []project_model.WorkflowFilter
		issue    *issues_model.Issue
		source   int64
		target   int64
		expected bool
	}{
		// --- no filters --------------------------------------------------------
		{
			name:     "no filters matches everything",
			event:    itemOpened,
			issue:    issue,
			expected: true,
		},

		// --- issue_type --------------------------------------------------------
		{
			name:     "issue_type empty means both",
			event:    itemOpened,
			filters:  []project_model.WorkflowFilter{{Type: project_model.WorkflowFilterTypeIssueType, Value: ""}},
			issue:    pull,
			expected: true,
		},
		{
			name:     "issue_type issue matches an issue",
			event:    itemOpened,
			filters:  []project_model.WorkflowFilter{{Type: project_model.WorkflowFilterTypeIssueType, Value: project_model.WorkflowIssueTypeIssue}},
			issue:    issue,
			expected: true,
		},
		{
			name:     "issue_type issue rejects a pull request",
			event:    itemOpened,
			filters:  []project_model.WorkflowFilter{{Type: project_model.WorkflowFilterTypeIssueType, Value: project_model.WorkflowIssueTypeIssue}},
			issue:    pull,
			expected: false,
		},
		{
			name:     "issue_type pull_request matches a pull request",
			event:    itemOpened,
			filters:  []project_model.WorkflowFilter{{Type: project_model.WorkflowFilterTypeIssueType, Value: project_model.WorkflowIssueTypePullRequest}},
			issue:    pull,
			expected: true,
		},
		{
			name:     "issue_type pull_request rejects an issue",
			event:    itemOpened,
			filters:  []project_model.WorkflowFilter{{Type: project_model.WorkflowFilterTypeIssueType, Value: project_model.WorkflowIssueTypePullRequest}},
			issue:    issue,
			expected: false,
		},
		{
			name:     "issue_type with an unknown value fails closed",
			event:    itemOpened,
			filters:  []project_model.WorkflowFilter{{Type: project_model.WorkflowFilterTypeIssueType, Value: "epic"}},
			issue:    issue,
			expected: false,
		},

		// --- target_column -----------------------------------------------------
		{
			name:     "target_column matches the resolved target",
			event:    columnChanged,
			filters:  []project_model.WorkflowFilter{{Type: project_model.WorkflowFilterTypeTargetColumn, Value: "20"}},
			issue:    issue,
			source:   sourceColumnID,
			target:   targetColumnID,
			expected: true,
		},
		{
			name:     "target_column rejects a different target",
			event:    columnChanged,
			filters:  []project_model.WorkflowFilter{{Type: project_model.WorkflowFilterTypeTargetColumn, Value: "21"}},
			issue:    issue,
			source:   sourceColumnID,
			target:   targetColumnID,
			expected: false,
		},
		{
			// The headline fail-closed case: before, a target column of 0 skipped the
			// filter entirely and the workflow matched every item.
			name:     "target_column fails closed when no target column was supplied",
			event:    columnChanged,
			filters:  []project_model.WorkflowFilter{{Type: project_model.WorkflowFilterTypeTargetColumn, Value: "20"}},
			issue:    issue,
			source:   sourceColumnID,
			target:   0,
			expected: false,
		},
		{
			name:     "target_column fails closed on an empty value",
			event:    columnChanged,
			filters:  []project_model.WorkflowFilter{{Type: project_model.WorkflowFilterTypeTargetColumn, Value: ""}},
			issue:    issue,
			source:   sourceColumnID,
			target:   targetColumnID,
			expected: false,
		},
		{
			name:     "target_column fails closed on an unparsable value",
			event:    columnChanged,
			filters:  []project_model.WorkflowFilter{{Type: project_model.WorkflowFilterTypeTargetColumn, Value: "not-a-number"}},
			issue:    issue,
			source:   sourceColumnID,
			target:   targetColumnID,
			expected: false,
		},
		{
			name:     "target_column fails closed on a zero value",
			event:    columnChanged,
			filters:  []project_model.WorkflowFilter{{Type: project_model.WorkflowFilterTypeTargetColumn, Value: "0"}},
			issue:    issue,
			source:   sourceColumnID,
			target:   targetColumnID,
			expected: false,
		},
		{
			name:     "target_column fails closed on a negative value",
			event:    columnChanged,
			filters:  []project_model.WorkflowFilter{{Type: project_model.WorkflowFilterTypeTargetColumn, Value: "-20"}},
			issue:    issue,
			source:   sourceColumnID,
			target:   targetColumnID,
			expected: false,
		},

		// --- source_column -----------------------------------------------------
		{
			// Guards the default-column path: IssueChangeProjectColumn resolves the
			// sentinel 0 to the project's real default column before matching, so a
			// real ID always arrives here and the filter must still match.
			name:     "source_column matches the resolved source",
			event:    columnChanged,
			filters:  []project_model.WorkflowFilter{{Type: project_model.WorkflowFilterTypeSourceColumn, Value: "10"}},
			issue:    issue,
			source:   sourceColumnID,
			target:   targetColumnID,
			expected: true,
		},
		{
			name:     "source_column rejects a different source",
			event:    columnChanged,
			filters:  []project_model.WorkflowFilter{{Type: project_model.WorkflowFilterTypeSourceColumn, Value: "11"}},
			issue:    issue,
			source:   sourceColumnID,
			target:   targetColumnID,
			expected: false,
		},
		{
			name:     "source_column fails closed when no source column was supplied",
			event:    columnChanged,
			filters:  []project_model.WorkflowFilter{{Type: project_model.WorkflowFilterTypeSourceColumn, Value: "10"}},
			issue:    issue,
			source:   0,
			target:   targetColumnID,
			expected: false,
		},
		{
			name:     "source_column fails closed on an unparsable value",
			event:    columnChanged,
			filters:  []project_model.WorkflowFilter{{Type: project_model.WorkflowFilterTypeSourceColumn, Value: "10x"}},
			issue:    issue,
			source:   sourceColumnID,
			target:   targetColumnID,
			expected: false,
		},

		// --- labels ------------------------------------------------------------
		{
			name:     "labels matches an issue carrying the label",
			event:    itemOpened,
			filters:  []project_model.WorkflowFilter{{Type: project_model.WorkflowFilterTypeLabels, Value: "30"}},
			issue:    issue,
			expected: true,
		},
		{
			name:     "labels rejects an issue without the label",
			event:    itemOpened,
			filters:  []project_model.WorkflowFilter{{Type: project_model.WorkflowFilterTypeLabels, Value: "30"}},
			issue:    unlabelled,
			expected: false,
		},
		{
			name:     "labels fails closed on an unparsable value",
			event:    itemOpened,
			filters:  []project_model.WorkflowFilter{{Type: project_model.WorkflowFilterTypeLabels, Value: "thirty"}},
			issue:    issue,
			expected: false,
		},
		{
			name:     "labels fails closed on a zero value",
			event:    itemOpened,
			filters:  []project_model.WorkflowFilter{{Type: project_model.WorkflowFilterTypeLabels, Value: "0"}},
			issue:    issue,
			expected: false,
		},
		{
			name:     "labels fails closed on a negative value",
			event:    itemOpened,
			filters:  []project_model.WorkflowFilter{{Type: project_model.WorkflowFilterTypeLabels, Value: "-30"}},
			issue:    issue,
			expected: false,
		},

		// --- capability gate ---------------------------------------------------
		{
			// item_opened never supplies column IDs, so a stored source_column filter
			// could only ever be skipped -- which used to widen the match to every
			// item. It must fail closed instead.
			name:     "filter type unsupported by the event fails closed",
			event:    itemOpened,
			filters:  []project_model.WorkflowFilter{{Type: project_model.WorkflowFilterTypeSourceColumn, Value: "10"}},
			issue:    issue,
			expected: false,
		},
		{
			// pull_request_merged only declares the labels filter.
			name:     "issue_type on a labels-only event fails closed",
			event:    project_model.WorkflowEventPullRequestMerged,
			filters:  []project_model.WorkflowFilter{{Type: project_model.WorkflowFilterTypeIssueType, Value: project_model.WorkflowIssueTypePullRequest}},
			issue:    pull,
			expected: false,
		},
		{
			name:     "unknown filter type fails closed",
			event:    itemOpened,
			filters:  []project_model.WorkflowFilter{{Type: project_model.WorkflowFilterType("assignee"), Value: "1"}},
			issue:    issue,
			expected: false,
		},

		// --- several filters are ANDed -----------------------------------------
		{
			name:  "all filters matching matches",
			event: columnChanged,
			filters: []project_model.WorkflowFilter{
				{Type: project_model.WorkflowFilterTypeIssueType, Value: project_model.WorkflowIssueTypeIssue},
				{Type: project_model.WorkflowFilterTypeSourceColumn, Value: "10"},
				{Type: project_model.WorkflowFilterTypeTargetColumn, Value: "20"},
				{Type: project_model.WorkflowFilterTypeLabels, Value: "30"},
			},
			issue:    issue,
			source:   sourceColumnID,
			target:   targetColumnID,
			expected: true,
		},
		{
			name:  "a single failing filter rejects the whole set",
			event: columnChanged,
			filters: []project_model.WorkflowFilter{
				{Type: project_model.WorkflowFilterTypeIssueType, Value: project_model.WorkflowIssueTypeIssue},
				{Type: project_model.WorkflowFilterTypeSourceColumn, Value: "10"},
				{Type: project_model.WorkflowFilterTypeLabels, Value: "31"},
			},
			issue:    issue,
			source:   sourceColumnID,
			target:   targetColumnID,
			expected: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workflow := &project_model.Workflow{
				ID:              1,
				WorkflowEvent:   tc.event,
				WorkflowFilters: tc.filters,
			}
			assert.Equal(t, tc.expected, matchWorkflowsFilters(workflow, tc.issue, tc.source, tc.target))
		})
	}
}
