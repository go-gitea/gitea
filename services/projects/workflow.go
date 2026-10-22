// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"slices"

	audit_model "gitea.dev/models/audit"
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	project_model "gitea.dev/models/project"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/optional"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/services/audit"
)

func CreateWorkflow(ctx context.Context, doer *user_model.User, project *project_model.Project, opts *api.CreateProjectWorkflowOption) (*project_model.Workflow, error) {
	wf := &project_model.Workflow{
		ProjectID:     project.ID,
		WorkflowEvent: project_model.WorkflowEvent(opts.Event),
		Enabled:       optional.FromPtr(opts.Enabled).ValueOrDefault(true),
	}
	setWorkflowUpdater(wf, doer)
	if !wf.WorkflowEvent.IsValid() {
		return nil, util.ErrorWrap(util.ErrUnprocessableContent, "invalid event: %s", opts.Event)
	}
	if err := setWorkflowRules(ctx, project, wf, &opts.Filters, &opts.Actions); err != nil {
		return nil, err
	}
	if err := checkWorkflowActionsPermission(ctx, doer, project, wf); err != nil {
		return nil, err
	}
	if err := project_model.CreateWorkflow(ctx, wf); err != nil {
		return nil, err
	}
	recordWorkflowAudit(ctx, doer, audit_model.ProjectWorkflowCreate, project, wf)
	return wf, nil
}

func UpdateWorkflow(ctx context.Context, doer *user_model.User, project *project_model.Project, wf *project_model.Workflow, opts *api.EditProjectWorkflowOption) error {
	cols := []string{"updater_id", "updater_doer_data"}
	if opts.Filters != nil || opts.Actions != nil {
		if err := setWorkflowRules(ctx, project, wf, opts.Filters, opts.Actions); err != nil {
			return err
		}
	}
	if opts.Filters != nil {
		cols = append(cols, "workflow_filters")
	}
	if opts.Actions != nil {
		cols = append(cols, "workflow_actions")
	}
	if opts.Enabled != nil {
		wf.Enabled = *opts.Enabled
		cols = append(cols, "enabled")
	}
	if err := checkWorkflowActionsPermission(ctx, doer, project, wf); err != nil {
		return err
	}
	setWorkflowUpdater(wf, doer)
	if err := project_model.UpdateWorkflow(ctx, wf, cols...); err != nil {
		return err
	}
	recordWorkflowAudit(ctx, doer, audit_model.ProjectWorkflowUpdate, project, wf)
	return nil
}

func DeleteWorkflow(ctx context.Context, doer *user_model.User, project *project_model.Project, wf *project_model.Workflow) error {
	if err := project_model.DeleteWorkflow(ctx, project.ID, wf.ID); err != nil {
		return err
	}
	recordWorkflowAudit(ctx, doer, audit_model.ProjectWorkflowDelete, project, wf)
	return nil
}

func recordWorkflowAudit(ctx context.Context, doer *user_model.User, action audit_model.Action, project *project_model.Project, wf *project_model.Workflow) {
	var scope audit_model.EntityRef
	if project.Type == project_model.TypeRepository {
		if err := project.LoadRepo(ctx); err != nil {
			log.Error("recordWorkflowAudit: LoadRepo: %v", err)
			scope = audit_model.EntityRef{Type: audit_model.ScopeRepository, ID: project.RepoID}
		} else {
			scope = audit.ScopeFromRepository(project.Repo)
		}
	} else {
		scope = audit.ScopeFromUserID(ctx, project.OwnerID)
	}
	audit.RecordAs(ctx, doer, action, scope, "project", project.Title, "project_id", project.ID, "workflow_id", wf.ID, "event", string(wf.WorkflowEvent))
}

func setWorkflowRules(ctx context.Context, project *project_model.Project, wf *project_model.Workflow, newFilters *api.ProjectWorkflowFilters, newActions *api.ProjectWorkflowActions) error {
	if newFilters != nil {
		wf.WorkflowFilters = project_model.WorkflowFilters(*newFilters)
		wf.WorkflowFilters.LabelIDs = uniqueIDs(newFilters.LabelIDs)
	}
	if newActions != nil {
		wf.WorkflowActions = project_model.WorkflowActions(*newActions)
		wf.WorkflowActions.AddLabelIDs = uniqueIDs(newActions.AddLabelIDs)
		wf.WorkflowActions.RemoveLabelIDs = uniqueIDs(newActions.RemoveLabelIDs)
	}
	filters, actions := wf.WorkflowFilters, wf.WorkflowActions

	if typ := wf.UnsupportedRule(); typ != "" {
		return util.ErrorWrap(util.ErrUnprocessableContent, "%s is not supported by this event", typ)
	}
	if actions.ColumnID == 0 && !actions.ChangesIssue() {
		return util.ErrorWrapTranslatable(util.ErrorWrap(util.ErrUnprocessableContent, "at least one action must be configured"), "projects.workflows.at_least_one_action_required")
	}
	if !slices.Contains([]string{"", project_model.WorkflowIssueTypeIssue, project_model.WorkflowIssueTypePullRequest}, filters.IssueType) {
		return util.ErrorWrap(util.ErrUnprocessableContent, "invalid issue_type: %s", filters.IssueType)
	}
	if !slices.Contains([]string{"", project_model.WorkflowIssueStateClose, project_model.WorkflowIssueStateReopen}, actions.IssueState) {
		return util.ErrorWrap(util.ErrUnprocessableContent, "invalid issue_state: %s", actions.IssueState)
	}
	if actions.IssueState == project_model.WorkflowIssueStateReopen && filters.IssueType == project_model.WorkflowIssueTypePullRequest {
		return util.ErrorWrap(util.ErrUnprocessableContent, "pull requests cannot be reopened by a workflow")
	}

	for _, columnID := range []int64{filters.SourceColumnID, filters.TargetColumnID, actions.ColumnID} {
		if columnID == 0 {
			continue
		}
		if _, err := project_model.GetColumnByIDAndProjectID(ctx, columnID, project.ID); project_model.IsErrProjectColumnNotExist(err) {
			return util.ErrorWrap(util.ErrUnprocessableContent, "invalid column: %d", columnID)
		} else if err != nil {
			return err
		}
	}

	labelIDs := slices.Concat(filters.LabelIDs, actions.AddLabelIDs, actions.RemoveLabelIDs)
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

func checkWorkflowActionsPermission(ctx context.Context, doer *user_model.User, project *project_model.Project, wf *project_model.Workflow) error {
	if !wf.Enabled || !wf.WorkflowActions.ChangesIssue() {
		return nil
	}
	if doer.ID <= 0 { // a system user's permissions belong to a task or key that may be gone when the workflow runs
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

func setWorkflowUpdater(wf *project_model.Workflow, doer *user_model.User) {
	wf.UpdaterID, wf.UpdaterDoerData = doer.ID, ""
	if doer.ExtDoerData != nil {
		wf.UpdaterDoerData = doer.ExtDoerData.EncodeToString()
	}
}

func canWorkflowChangeIssue(ctx context.Context, wf *project_model.Workflow, issue *issues_model.Issue) (bool, error) {
	updater, err := user_model.GetDoerPermissionUser(ctx, wf.UpdaterID, wf.UpdaterDoerData)
	if user_model.IsErrUserNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if !updater.IsActive || updater.ProhibitLogin {
		return false, nil
	}
	perm, err := access_model.GetDoerRepoPermission(ctx, issue.Repo, updater)
	if err != nil {
		return false, err
	}
	return perm.CanWriteIssuesOrPulls(issue.IsPull), nil
}
