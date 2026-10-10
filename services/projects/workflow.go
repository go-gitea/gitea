// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"slices"

	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	project_model "gitea.dev/models/project"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/optional"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
)

// CreateWorkflow validates and stores a new workflow whose label and issue state actions run as doer
func CreateWorkflow(ctx context.Context, doer *user_model.User, project *project_model.Project, opts *api.CreateProjectWorkflowOption) (*project_model.Workflow, error) {
	wf := &project_model.Workflow{
		ProjectID:     project.ID,
		WorkflowEvent: project_model.WorkflowEvent(opts.Event),
		Enabled:       optional.FromPtr(opts.Enabled).ValueOrDefault(true),
		UpdaterID:     doer.ID,
	}
	if !wf.WorkflowEvent.IsValid() {
		return nil, util.ErrorWrap(util.ErrUnprocessableContent, "invalid event: %s", opts.Event)
	}
	if err := setWorkflowRules(ctx, project, wf, &opts.Filters, &opts.Actions); err != nil {
		return nil, err
	}
	if err := checkWorkflowActionsPermission(ctx, doer, project, wf); err != nil {
		return nil, err
	}
	return wf, project_model.CreateWorkflow(ctx, wf)
}

// UpdateWorkflow applies the given changes, afterwards the workflow's actions run as doer
func UpdateWorkflow(ctx context.Context, doer *user_model.User, project *project_model.Project, wf *project_model.Workflow, opts *api.EditProjectWorkflowOption) error {
	cols := []string{"updater_id"}
	if opts.Filters != nil || opts.Actions != nil {
		if err := setWorkflowRules(ctx, project, wf, opts.Filters, opts.Actions); err != nil {
			return err
		}
		cols = append(cols, "workflow_filters", "workflow_actions")
	}
	if opts.Enabled != nil {
		wf.Enabled = *opts.Enabled
		cols = append(cols, "enabled")
	}
	if err := checkWorkflowActionsPermission(ctx, doer, project, wf); err != nil {
		return err
	}
	wf.UpdaterID = doer.ID
	return project_model.UpdateWorkflow(ctx, wf, cols...)
}

// setWorkflowRules replaces the given non-nil rules, rejecting anything that cannot be resolved
// because dropping it would make the workflow match or do more than configured
func setWorkflowRules(ctx context.Context, project *project_model.Project, wf *project_model.Workflow, filters *api.ProjectWorkflowFilters, actions *api.ProjectWorkflowActions) error {
	if filters != nil {
		wf.WorkflowFilters = project_model.WorkflowFilters(*filters)
		wf.WorkflowFilters.LabelIDs = uniqueIDs(filters.LabelIDs)
	}
	if actions != nil {
		wf.WorkflowActions = project_model.WorkflowActions(*actions)
		wf.WorkflowActions.AddLabelIDs = uniqueIDs(actions.AddLabelIDs)
		wf.WorkflowActions.RemoveLabelIDs = uniqueIDs(actions.RemoveLabelIDs)
	}
	f, a := wf.WorkflowFilters, wf.WorkflowActions

	if typ := wf.UnsupportedRule(); typ != "" {
		return util.ErrorWrap(util.ErrUnprocessableContent, "%s is not supported by this event", typ)
	}
	if len(a.Types()) == 0 {
		return util.ErrorWrapTranslatable(util.ErrorWrap(util.ErrUnprocessableContent, "at least one action must be configured"), "projects.workflows.at_least_one_action_required")
	}
	if f.IssueType != "" && f.IssueType != project_model.WorkflowIssueTypeIssue && f.IssueType != project_model.WorkflowIssueTypePullRequest {
		return util.ErrorWrap(util.ErrUnprocessableContent, "invalid issue_type: %s", f.IssueType)
	}
	if a.IssueState != "" && a.IssueState != project_model.WorkflowIssueStateClose && a.IssueState != project_model.WorkflowIssueStateReopen {
		return util.ErrorWrap(util.ErrUnprocessableContent, "invalid issue_state: %s", a.IssueState)
	}

	for _, columnID := range []int64{f.SourceColumnID, f.TargetColumnID, a.ColumnID} {
		if columnID == 0 {
			continue
		}
		if _, err := project_model.GetColumnByIDAndProjectID(ctx, columnID, project.ID); project_model.IsErrProjectColumnNotExist(err) {
			return util.ErrorWrap(util.ErrUnprocessableContent, "invalid column: %d", columnID)
		} else if err != nil {
			return err
		}
	}

	labelIDs := slices.Concat(f.LabelIDs, a.AddLabelIDs, a.RemoveLabelIDs)
	if len(labelIDs) == 0 {
		return nil
	}
	labels, err := GetProjectLabels(ctx, project)
	if err != nil {
		return err
	}
	for _, labelID := range labelIDs {
		if !slices.ContainsFunc(labels, func(l *issues_model.Label) bool { return l.ID == labelID }) {
			return util.ErrorWrap(util.ErrUnprocessableContent, "invalid label: %d", labelID)
		}
	}
	return nil
}

func uniqueIDs(ids []int64) []int64 {
	return slices.Compact(slices.Sorted(slices.Values(ids)))
}

// checkWorkflowActionsPermission rejects enabled label and issue state actions from users who could not make
// those changes themselves. Owner-level projects span repositories, they are checked when the workflow runs.
func checkWorkflowActionsPermission(ctx context.Context, doer *user_model.User, project *project_model.Project, wf *project_model.Workflow) error {
	if !wf.Enabled || !wf.WorkflowActions.ChangesIssue() {
		return nil
	}
	if doer.ID <= 0 { // system users can't be restored with their permissions when the workflow runs
		return util.NewPermissionDeniedErrorf("label and issue state actions must be saved by a user")
	}
	if project.Type != project_model.TypeRepository {
		return nil
	}
	if err := project.LoadRepo(ctx); err != nil {
		return err
	}
	perm, err := access_model.GetDoerRepoPermission(ctx, project.Repo, doer)
	if err != nil {
		return err
	}
	if !perm.CanWrite(unit.TypeIssues) && !perm.CanWrite(unit.TypePullRequests) {
		return util.NewPermissionDeniedErrorf("label and issue state actions require write access to issues or pull requests")
	}
	return nil
}

// canWorkflowChangeIssue reports whether the workflow's updater may change labels or the state of the issue
func canWorkflowChangeIssue(ctx context.Context, wf *project_model.Workflow, issue *issues_model.Issue) (bool, error) {
	updater, err := user_model.GetUserByID(ctx, wf.UpdaterID)
	if user_model.IsErrUserNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	perm, err := access_model.GetDoerRepoPermission(ctx, issue.Repo, updater)
	if err != nil {
		return false, err
	}
	return perm.CanWriteIssuesOrPulls(issue.IsPull), nil
}
