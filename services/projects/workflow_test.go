// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"sync"
	"testing"

	issues_model "gitea.dev/models/issues"
	project_model "gitea.dev/models/project"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/optional"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	notify_service "gitea.dev/services/notify"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchWorkflowFilters(t *testing.T) {
	issue := &issues_model.Issue{Labels: []*issues_model.Label{{ID: 1}}}
	pull := &issues_model.Issue{IsPull: true}
	moved := workflowTrigger{sourceColumnID: 1, targetColumnID: 2}
	opened, columnChanged := project_model.WorkflowEventItemOpened, project_model.WorkflowEventItemColumnChanged
	tests := []struct {
		name    string
		event   project_model.WorkflowEvent
		filters project_model.WorkflowFilters
		issue   *issues_model.Issue
		trigger workflowTrigger
		want    bool
	}{
		{"no filters", opened, project_model.WorkflowFilters{}, issue, workflowTrigger{}, true},
		{"issue type", opened, project_model.WorkflowFilters{IssueType: "issue"}, issue, workflowTrigger{}, true},
		{"other issue type", opened, project_model.WorkflowFilters{IssueType: "issue"}, pull, workflowTrigger{}, false},
		{"unknown issue type", opened, project_model.WorkflowFilters{IssueType: "bogus"}, issue, workflowTrigger{}, false},
		{"label", opened, project_model.WorkflowFilters{LabelIDs: []int64{1}}, issue, workflowTrigger{}, true},
		{"missing label", opened, project_model.WorkflowFilters{LabelIDs: []int64{1, 2}}, issue, workflowTrigger{}, false},
		{"columns", columnChanged, project_model.WorkflowFilters{SourceColumnID: 1, TargetColumnID: 2}, issue, moved, true},
		{"other target", columnChanged, project_model.WorkflowFilters{TargetColumnID: 3}, issue, moved, false},
		{"no column supplied", columnChanged, project_model.WorkflowFilters{SourceColumnID: 1}, issue, workflowTrigger{}, false},
		{"unsupported by event", opened, project_model.WorkflowFilters{TargetColumnID: 2}, issue, moved, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wf := &project_model.Workflow{WorkflowEvent: test.event, WorkflowFilters: test.filters}
			assert.Equal(t, test.want, matchWorkflowFilters(wf, test.issue, test.trigger))
		})
	}
}

func TestCreateUpdateWorkflow(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	reader := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	project := unittest.AssertExistsAndLoadBean(t, &project_model.Project{ID: 1})

	t.Run("Rejects", func(t *testing.T) {
		tests := []struct {
			name string
			doer *user_model.User
			opts api.CreateProjectWorkflowOption
			code int
		}{
			{"unknown event", owner, api.CreateProjectWorkflowOption{Event: "bogus", Actions: api.ProjectWorkflowActions{ColumnID: 2}}, 422},
			{"no actions", owner, api.CreateProjectWorkflowOption{Event: "item_opened"}, 422},
			{"unknown column", owner, api.CreateProjectWorkflowOption{Event: "item_opened", Actions: api.ProjectWorkflowActions{ColumnID: 999}}, 422},
			{"other project column", owner, api.CreateProjectWorkflowOption{Event: "item_opened", Actions: api.ProjectWorkflowActions{ColumnID: 4}}, 422},
			{"other owner label", owner, api.CreateProjectWorkflowOption{Event: "item_opened", Actions: api.ProjectWorkflowActions{AddLabelIDs: []int64{3}}}, 422},
			{"unsupported filter", owner, api.CreateProjectWorkflowOption{Event: "item_opened", Filters: api.ProjectWorkflowFilters{SourceColumnID: 1}, Actions: api.ProjectWorkflowActions{ColumnID: 2}}, 422},
			{"unsupported action", owner, api.CreateProjectWorkflowOption{Event: "item_opened", Actions: api.ProjectWorkflowActions{IssueState: "close"}}, 422},
			{"unknown issue state", owner, api.CreateProjectWorkflowOption{Event: "item_added_to_project", Actions: api.ProjectWorkflowActions{IssueState: "bogus"}}, 422},
			{"unknown issue type", owner, api.CreateProjectWorkflowOption{Event: "item_opened", Filters: api.ProjectWorkflowFilters{IssueType: "bogus"}, Actions: api.ProjectWorkflowActions{ColumnID: 2}}, 422},
			{"labels without issue write", reader, api.CreateProjectWorkflowOption{Event: "item_opened", Actions: api.ProjectWorkflowActions{AddLabelIDs: []int64{1}}}, 403},
			{"labels by a system user", user_model.NewActionsUserWithTaskID(1), api.CreateProjectWorkflowOption{Event: "item_opened", Actions: api.ProjectWorkflowActions{AddLabelIDs: []int64{1}}}, 403},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				_, err := CreateWorkflow(ctx, test.doer, project, &test.opts)
				_, code := util.ErrorUnwrapForUser(err)
				assert.Equal(t, test.code, code)
			})
		}
	})

	t.Run("PartialUpdate", func(t *testing.T) {
		wf, err := CreateWorkflow(ctx, reader, project, &api.CreateProjectWorkflowOption{
			Event:   "item_opened",
			Filters: api.ProjectWorkflowFilters{IssueType: "issue", LabelIDs: []int64{2, 1, 2}},
			Actions: api.ProjectWorkflowActions{ColumnID: 2},
		})
		require.NoError(t, err)

		require.NoError(t, UpdateWorkflow(ctx, owner, project, wf, &api.EditProjectWorkflowOption{Enabled: new(false)}))
		require.NoError(t, UpdateWorkflow(ctx, owner, project, wf, &api.EditProjectWorkflowOption{Actions: &api.ProjectWorkflowActions{AddLabelIDs: []int64{1}}}))

		err = UpdateWorkflow(ctx, reader, project, wf, &api.EditProjectWorkflowOption{Enabled: new(true)})
		_, code := util.ErrorUnwrapForUser(err)
		assert.Equal(t, 403, code, "re-enabling label actions needs issue write")

		stored := unittest.AssertExistsAndLoadBean(t, &project_model.Workflow{ID: wf.ID})
		assert.False(t, stored.Enabled)
		assert.Equal(t, project_model.WorkflowFilters{IssueType: "issue", LabelIDs: []int64{1, 2}}, stored.WorkflowFilters)
		assert.Equal(t, project_model.WorkflowActions{AddLabelIDs: []int64{1}}, stored.WorkflowActions)
		assert.Equal(t, owner.ID, stored.UpdaterID)
	})
}

type labelDoerRecorder struct {
	notify_service.NullNotifier
	doer *user_model.User
}

func (r *labelDoerRecorder) IssueChangeLabels(_ context.Context, doer *user_model.User, _ *issues_model.Issue, _, _ []*issues_model.Label) {
	r.doer = doer
}

var registerLabelDoerRecorder = sync.OnceValue(func() *labelDoerRecorder {
	r := &labelDoerRecorder{}
	notify_service.RegisterNotifier(r)
	return r
})

func TestWorkflowExecution(t *testing.T) {
	const labelID = 2
	require.NoError(t, unittest.PrepareTestDatabase())
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	addLabel := project_model.WorkflowActions{AddLabelIDs: []int64{labelID}}
	setup := func(t *testing.T, event project_model.WorkflowEvent, updaterID int64, filters project_model.WorkflowFilters, actions project_model.WorkflowActions) {
		require.NoError(t, unittest.PrepareTestDatabase())
		require.NoError(t, project_model.CreateWorkflow(t.Context(), &project_model.Workflow{
			ProjectID:       1,
			WorkflowEvent:   event,
			WorkflowFilters: filters,
			WorkflowActions: actions,
			Enabled:         true,
			UpdaterID:       updaterID,
		}))
	}
	hasLabel := func(t *testing.T, issueID int64) bool {
		return unittest.GetCount(t, &issues_model.IssueLabel{IssueID: issueID, LabelID: labelID}) == 1
	}
	moveIssue := func(t *testing.T, ctx context.Context, issueID, columnID int64) {
		issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: issueID})
		column := unittest.AssertExistsAndLoadBean(t, &project_model.Column{ID: columnID})
		require.NoError(t, MoveIssueToColumn(ctx, doer, issue, column, optional.None[int64]()))
	}

	t.Run("ColumnChangedFromDefaultColumn", func(t *testing.T) {
		setup(t, project_model.WorkflowEventItemColumnChanged, doer.ID, project_model.WorkflowFilters{SourceColumnID: 1, TargetColumnID: 2}, addLabel)
		moveIssue(t, t.Context(), 2, 2)
		assert.True(t, hasLabel(t, 2))
		comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: 2, Type: issues_model.CommentTypeLabel, LabelID: labelID})
		assert.Equal(t, issues_model.SpecialDoerNameProjectWorkflow, comment.CommentMetaData.SpecialDoerName)
	})

	t.Run("NoOpMoveWithinDefaultColumn", func(t *testing.T) {
		setup(t, project_model.WorkflowEventItemColumnChanged, doer.ID, project_model.WorkflowFilters{}, addLabel)
		moveIssue(t, t.Context(), 2, 1)
		assert.False(t, hasLabel(t, 2))
	})

	t.Run("NoCascadeFromWorkflows", func(t *testing.T) {
		setup(t, project_model.WorkflowEventItemColumnChanged, doer.ID, project_model.WorkflowFilters{}, addLabel)
		moveIssue(t, issues_model.WithProjectWorkflow(t.Context(), project_model.WorkflowEventItemOpened), 1, 2)
		assert.False(t, hasLabel(t, 1))
	})

	t.Run("UpdaterWithoutIssueWrite", func(t *testing.T) {
		setup(t, project_model.WorkflowEventItemColumnChanged, 5, project_model.WorkflowFilters{}, addLabel)
		moveIssue(t, t.Context(), 1, 2)
		assert.False(t, hasLabel(t, 1))
	})

	t.Run("OfficialReviewsOnly", func(t *testing.T) {
		setup(t, project_model.WorkflowEventCodeReviewApproved, doer.ID, project_model.WorkflowFilters{}, addLabel)
		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{IssueID: 3})
		review := &issues_model.Review{Type: issues_model.ReviewTypeApprove, Reviewer: doer}
		(&workflowNotifier{}).PullRequestReview(t.Context(), pr, review, nil, nil)
		assert.False(t, hasLabel(t, 3))
		review.Official = true
		(&workflowNotifier{}).PullRequestReview(t.Context(), pr, review, nil, nil)
		assert.True(t, hasLabel(t, 3))
	})

	t.Run("AddedToAndRemovedFromProject", func(t *testing.T) {
		setup(t, project_model.WorkflowEventItemAddedToProject, doer.ID, project_model.WorkflowFilters{}, addLabel)
		require.NoError(t, project_model.CreateWorkflow(t.Context(), &project_model.Workflow{
			ProjectID:       1,
			WorkflowEvent:   project_model.WorkflowEventItemRemovedFromProject,
			WorkflowActions: project_model.WorkflowActions{RemoveLabelIDs: []int64{labelID}},
			Enabled:         true,
			UpdaterID:       doer.ID,
		}))
		issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 11})
		column := unittest.AssertExistsAndLoadBean(t, &project_model.Column{ID: 2})
		require.NoError(t, AddIssueToColumn(t.Context(), doer, issue, column))
		assert.True(t, hasLabel(t, 11))
		require.NoError(t, RemoveIssueFromColumn(t.Context(), doer, issue, column))
		assert.False(t, hasLabel(t, 11))
	})

	t.Run("ActionsUserDoerKeepsTask", func(t *testing.T) {
		recorder := registerLabelDoerRecorder()
		setup(t, project_model.WorkflowEventItemClosed, doer.ID, project_model.WorkflowFilters{}, addLabel)
		issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
		(&workflowNotifier{}).IssueChangeStatus(t.Context(), user_model.NewActionsUserWithTaskID(7), "", issue, nil, true)
		require.True(t, hasLabel(t, 1))
		taskID, ok := user_model.GetActionsUserTaskID(recorder.doer)
		assert.True(t, ok)
		assert.EqualValues(t, 7, taskID)
	})
}
