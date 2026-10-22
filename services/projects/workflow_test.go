// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"testing"

	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/organization"
	"gitea.dev/models/perm"
	project_model "gitea.dev/models/project"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	"gitea.dev/modules/util"
	issue_service "gitea.dev/services/issue"
	notify_service "gitea.dev/services/notify"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchWorkflowFilters(t *testing.T) {
	issue := &issues_model.Issue{Labels: []*issues_model.Label{{ID: 1}}}
	pull := &issues_model.Issue{IsPull: true}
	moved := workflowTrigger{sourceColumnID: 1, targetColumnID: 2}
	added, columnChanged := project_model.WorkflowEventItemAddedToProject, project_model.WorkflowEventItemColumnChanged
	tests := []struct {
		name    string
		event   project_model.WorkflowEvent
		filters project_model.WorkflowFilters
		issue   *issues_model.Issue
		trigger workflowTrigger
		want    bool
	}{
		{"no filters", added, project_model.WorkflowFilters{}, issue, workflowTrigger{}, true},
		{"issue type", added, project_model.WorkflowFilters{IssueType: "issue"}, issue, workflowTrigger{}, true},
		{"other issue type", added, project_model.WorkflowFilters{IssueType: "issue"}, pull, workflowTrigger{}, false},
		{"label", added, project_model.WorkflowFilters{LabelIDs: []int64{1}}, issue, workflowTrigger{}, true},
		{"missing label", added, project_model.WorkflowFilters{LabelIDs: []int64{1, 2}}, issue, workflowTrigger{}, false},
		{"columns", columnChanged, project_model.WorkflowFilters{SourceColumnID: 1, TargetColumnID: 2}, issue, moved, true},
		{"other target", columnChanged, project_model.WorkflowFilters{TargetColumnID: 3}, issue, moved, false},
		{"no column supplied", columnChanged, project_model.WorkflowFilters{SourceColumnID: 1}, issue, workflowTrigger{}, false},
		{"unsupported by event", added, project_model.WorkflowFilters{TargetColumnID: 2}, issue, moved, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wf := &project_model.Workflow{WorkflowEvent: test.event, WorkflowFilters: test.filters}
			assert.Equal(t, test.want, matchWorkflowFilters(wf, test.issue, test.trigger))
		})
	}
}

func TestCanManageWorkflows(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	user := func(id int64) *user_model.User { return unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: id}) }
	publicOnly := func(id int64) *user_model.User {
		doer := *user(id)
		doer.ExtDoerData = user_model.NewTokenExtDoerData(user_model.CredentialAccessToken, 1, "public-only,write:issue")
		return &doer
	}
	repoProject := unittest.AssertExistsAndLoadBean(t, &project_model.Project{ID: 2})
	orgProject := &project_model.Project{Title: "org", OwnerID: 3, Type: project_model.TypeOrganization}
	userProject := &project_model.Project{Title: "user", OwnerID: 2, Type: project_model.TypeIndividual}
	for _, tc := range []struct {
		project *project_model.Project
		doer    *user_model.User
		want    bool
	}{
		{repoProject, user(4), true},
		{repoProject, user(5), false},
		{orgProject, user(2), true},
		{orgProject, user(1), true},
		{orgProject, user(4), false},
		{orgProject, publicOnly(2), false},
		{userProject, user(2), true},
		{userProject, user(1), false},
		{userProject, publicOnly(2), false},
	} {
		allowed, err := CanManageWorkflows(t.Context(), tc.doer, tc.project)
		require.NoError(t, err)
		assert.Equal(t, tc.want, allowed, "project %q, user %d", tc.project.Title, tc.doer.ID)
	}
	for _, removedUnit := range []unit.Type{unit.TypeIssues, unit.TypePullRequests} {
		require.NoError(t, unittest.PrepareTestDatabase())
		_, err := db.Exec(t.Context(), "UPDATE team SET authorize = ? WHERE id = ?", perm.AccessModeNone, 2)
		require.NoError(t, err)
		_, err = db.DeleteByBean(t.Context(), &organization.TeamUnit{TeamID: 2, Type: removedUnit})
		require.NoError(t, err)
		allowed, err := CanManageWorkflows(t.Context(), user(4), repoProject)
		require.NoError(t, err)
		assert.False(t, allowed, removedUnit.LogString())
	}
}

func TestCreateUpdateWorkflow(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.Audit.RecordOutput, setting.AuditRecordOutputDatabase)()
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
			{"no actions", owner, api.CreateProjectWorkflowOption{Event: "item_added_to_project"}, 422},
			{"other project column", owner, api.CreateProjectWorkflowOption{Event: "item_added_to_project", Actions: api.ProjectWorkflowActions{ColumnID: 4}}, 422},
			{"other owner label", owner, api.CreateProjectWorkflowOption{Event: "item_added_to_project", Actions: api.ProjectWorkflowActions{AddLabelIDs: []int64{3}}}, 422},
			{"unsupported filter", owner, api.CreateProjectWorkflowOption{Event: "item_added_to_project", Filters: api.ProjectWorkflowFilters{SourceColumnID: 1}, Actions: api.ProjectWorkflowActions{ColumnID: 2}}, 422},
			{"unsupported action", owner, api.CreateProjectWorkflowOption{Event: "item_removed_from_project", Actions: api.ProjectWorkflowActions{ColumnID: 2}}, 422},
			{"unknown issue state", owner, api.CreateProjectWorkflowOption{Event: "item_added_to_project", Actions: api.ProjectWorkflowActions{IssueState: "bogus"}}, 422},
			{"reopen pull requests", owner, api.CreateProjectWorkflowOption{Event: "item_added_to_project", Filters: api.ProjectWorkflowFilters{IssueType: "pull_request"}, Actions: api.ProjectWorkflowActions{IssueState: "reopen"}}, 422},
			{"unknown issue type", owner, api.CreateProjectWorkflowOption{Event: "item_added_to_project", Filters: api.ProjectWorkflowFilters{IssueType: "bogus"}, Actions: api.ProjectWorkflowActions{ColumnID: 2}}, 422},
			{"without issue and pull request write", reader, api.CreateProjectWorkflowOption{Event: "item_added_to_project", Actions: api.ProjectWorkflowActions{ColumnID: 2}}, 403},
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
		wf, err := CreateWorkflow(ctx, owner, project, &api.CreateProjectWorkflowOption{
			Event:   "item_added_to_project",
			Filters: api.ProjectWorkflowFilters{IssueType: "issue", LabelIDs: []int64{2, 1, 2}},
			Actions: api.ProjectWorkflowActions{ColumnID: 2},
		})
		require.NoError(t, err)

		require.NoError(t, UpdateWorkflow(ctx, owner, project, wf, &api.EditProjectWorkflowOption{Enabled: new(false)}))
		require.NoError(t, UpdateWorkflow(ctx, owner, project, wf, &api.EditProjectWorkflowOption{Actions: &api.ProjectWorkflowActions{AddLabelIDs: []int64{1}}}))

		unittest.AssertExistsAndLoadBean(t, &audit_model.Event{Action: audit_model.ProjectWorkflowCreate, ActorID: owner.ID, ScopeType: audit_model.ScopeRepository, ScopeID: project.RepoID})

		stored := unittest.AssertExistsAndLoadBean(t, &project_model.Workflow{ID: wf.ID})
		assert.False(t, stored.Enabled)
		assert.Equal(t, project_model.WorkflowFilters{IssueType: "issue", LabelIDs: []int64{1, 2}}, stored.WorkflowFilters)
		assert.Equal(t, project_model.WorkflowActions{AddLabelIDs: []int64{1}}, stored.WorkflowActions)

		assert.ErrorIs(t, UpdateWorkflow(ctx, reader, project, stored, &api.EditProjectWorkflowOption{Enabled: new(true)}), util.ErrPermissionDenied)
		assert.ErrorIs(t, DeleteWorkflow(ctx, reader, project, stored), util.ErrPermissionDenied)
		require.NoError(t, DeleteWorkflow(ctx, owner, project, stored))
		unittest.AssertNotExistsBean(t, &project_model.Workflow{ID: wf.ID})
		for _, action := range []audit_model.Action{audit_model.ProjectWorkflowUpdate, audit_model.ProjectWorkflowDelete} {
			unittest.AssertExistsAndLoadBean(t, &audit_model.Event{Action: action, ActorID: owner.ID, ScopeType: audit_model.ScopeRepository, ScopeID: project.RepoID})
		}
	})

	userProject := &project_model.Project{Title: "workflows", OwnerID: owner.ID, CreatorID: owner.ID, Type: project_model.TypeIndividual}
	require.NoError(t, project_model.NewProject(ctx, userProject))
	_, err := CreateWorkflow(ctx, owner, userProject, &api.CreateProjectWorkflowOption{Event: "item_added_to_project", Actions: api.ProjectWorkflowActions{IssueState: "close"}})
	require.NoError(t, err)
	event := unittest.AssertExistsAndLoadBean(t, &audit_model.Event{Action: audit_model.ProjectWorkflowCreate, ScopeType: audit_model.ScopeUser, ScopeID: userProject.OwnerID})
	assert.Contains(t, event.Message, "item_added_to_project")
}

type labelChangeRecorder struct {
	notify_service.NullNotifier
	added [][]int64
}

func (r *labelChangeRecorder) IssueChangeLabels(_ctx context.Context, _doer *user_model.User, _issue *issues_model.Issue, added, _removed []*issues_model.Label) {
	r.added = append(r.added, container.FilterSlice(added, func(l *issues_model.Label) (int64, bool) { return l.ID, true }))
}

func TestWorkflowExecution(t *testing.T) {
	const labelID = 2
	require.NoError(t, unittest.PrepareTestDatabase())
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	addLabel := project_model.WorkflowActions{AddLabelIDs: []int64{labelID}}
	addWorkflow := func(t *testing.T, wf project_model.Workflow) {
		wf.ProjectID, wf.Enabled = 1, true
		require.NoError(t, project_model.CreateWorkflow(t.Context(), &wf))
	}
	setup := func(t *testing.T, wf project_model.Workflow) {
		require.NoError(t, unittest.PrepareTestDatabase())
		addWorkflow(t, wf)
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
		setup(t, project_model.Workflow{WorkflowEvent: project_model.WorkflowEventItemColumnChanged, WorkflowFilters: project_model.WorkflowFilters{SourceColumnID: 1, TargetColumnID: 2}, WorkflowActions: addLabel})
		moveIssue(t, t.Context(), 2, 2)
		assert.True(t, hasLabel(t, 2))
	})

	t.Run("NoOpMoveWithinDefaultColumn", func(t *testing.T) {
		setup(t, project_model.Workflow{WorkflowEvent: project_model.WorkflowEventItemColumnChanged, WorkflowActions: addLabel})
		moveIssue(t, t.Context(), 2, 1)
		assert.False(t, hasLabel(t, 2))
		unittest.AssertNotExistsBean(t, &issues_model.Comment{IssueID: 2, Type: issues_model.CommentTypeProjectColumn})
	})

	t.Run("NoCascadeFromWorkflows", func(t *testing.T) {
		setup(t, project_model.Workflow{WorkflowEvent: project_model.WorkflowEventItemColumnChanged, WorkflowActions: addLabel})
		moveIssue(t, issues_model.WithProjectWorkflow(t.Context(), project_model.WorkflowEventItemAddedToProject), 1, 2)
		assert.False(t, hasLabel(t, 1))
	})

	t.Run("LabelsAndClosesPullRequestAsTriggeringReader", func(t *testing.T) {
		setup(t, project_model.Workflow{WorkflowEvent: project_model.WorkflowEventItemColumnChanged, WorkflowActions: project_model.WorkflowActions{AddLabelIDs: []int64{labelID}, IssueState: project_model.WorkflowIssueStateClose}})
		reader := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
		require.NoError(t, MoveIssueToColumn(t.Context(), reader, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 3}), unittest.AssertExistsAndLoadBean(t, &project_model.Column{ID: 3}), optional.None[int64]()))
		assert.True(t, hasLabel(t, 3))
		assert.True(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 3}).IsClosed)
		for _, commentType := range []issues_model.CommentType{issues_model.CommentTypeLabel, issues_model.CommentTypeClose} {
			comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: 3, Type: commentType})
			assert.Equal(t, reader.ID, comment.PosterID)
			assert.Equal(t, &issues_model.CommentMetaData{
				ProjectWorkflowEvent: project_model.WorkflowEventItemColumnChanged,
				SpecialDoerName:      issues_model.SpecialDoerNameProjectWorkflow,
			}, comment.CommentMetaData)
		}
	})

	t.Run("ArchivedRepository", func(t *testing.T) {
		setup(t, project_model.Workflow{WorkflowEvent: project_model.WorkflowEventItemClosed, WorkflowActions: addLabel})
		require.NoError(t, repo_model.UpdateRepositoryColsNoAutoTime(t.Context(), &repo_model.Repository{ID: 1, IsArchived: true}, "is_archived"))
		(&workflowNotifier{}).IssueChangeStatus(t.Context(), doer, "", unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1}), nil, true)
		assert.False(t, hasLabel(t, 1))
	})

	t.Run("ReopenSkipsPullRequests", func(t *testing.T) {
		setup(t, project_model.Workflow{WorkflowEvent: project_model.WorkflowEventItemColumnChanged, WorkflowActions: project_model.WorkflowActions{IssueState: project_model.WorkflowIssueStateReopen}})
		require.NoError(t, issues_model.UpdateIssueCols(t.Context(), &issues_model.Issue{ID: 3, IsClosed: true}, "is_closed"))
		moveIssue(t, t.Context(), 3, 1)
		moveIssue(t, t.Context(), 5, 1)
		assert.True(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 3}).IsClosed)
		assert.False(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 5}).IsClosed)
	})

	t.Run("OfficialReviewsOnly", func(t *testing.T) {
		setup(t, project_model.Workflow{WorkflowEvent: project_model.WorkflowEventCodeReviewApproved, WorkflowActions: addLabel})
		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{IssueID: 3})
		review := &issues_model.Review{Type: issues_model.ReviewTypeApprove, Reviewer: doer}
		(&workflowNotifier{}).PullRequestReview(t.Context(), pr, review, nil, nil)
		assert.False(t, hasLabel(t, 3))
		review.Official = true
		(&workflowNotifier{}).PullRequestReview(t.Context(), pr, review, nil, nil)
		assert.True(t, hasLabel(t, 3))
	})

	t.Run("AddedToAndRemovedFromProject", func(t *testing.T) {
		setup(t, project_model.Workflow{WorkflowEvent: project_model.WorkflowEventItemAddedToProject, WorkflowActions: addLabel})
		addWorkflow(t, project_model.Workflow{WorkflowEvent: project_model.WorkflowEventItemRemovedFromProject, WorkflowActions: project_model.WorkflowActions{RemoveLabelIDs: []int64{labelID}}})
		issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 11})
		column := unittest.AssertExistsAndLoadBean(t, &project_model.Column{ID: 2})
		require.NoError(t, AddIssueToColumn(t.Context(), doer, issue, column))
		assert.True(t, hasLabel(t, 11))
		require.NoError(t, RemoveIssueFromColumn(t.Context(), doer, issue, column))
		assert.False(t, hasLabel(t, 11))
	})

	t.Run("NewItemInProject", func(t *testing.T) {
		setup(t, project_model.Workflow{WorkflowEvent: project_model.WorkflowEventItemAddedToProject, WorkflowActions: project_model.WorkflowActions{ColumnID: 3, AddLabelIDs: []int64{labelID}}})
		*labelChanges = labelChangeRecorder{}
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		issue := &issues_model.Issue{RepoID: repo.ID, PosterID: doer.ID, Title: "new item"}
		require.NoError(t, issue_service.NewIssue(t.Context(), repo, issue, []int64{1}, nil, nil, []int64{1}))
		assert.Equal(t, [][]int64{{1}, {labelID}}, labelChanges.added)
		assert.EqualValues(t, 3, unittest.AssertExistsAndLoadBean(t, &project_model.ProjectIssue{IssueID: issue.ID}).ProjectColumnID)
	})
}
