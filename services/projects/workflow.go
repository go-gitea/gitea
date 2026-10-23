// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"slices"

	audit_model "gitea.dev/models/audit"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/organization"
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

// CanManageWorkflows checks the authority needed besides project write to manage a project's workflows. They act on
// the project's items without further permission checks, so this needs write access to both issues and pull requests
// of a repository project's repository, owning an organization project's organization, or being a user project's user.
func CanManageWorkflows(ctx context.Context, doer *user_model.User, project *project_model.Project) (bool, error) {
	if project.Type == project_model.TypeRepository {
		if err := project.LoadRepo(ctx); err != nil {
			return false, err
		}
		perm, err := access_model.GetDoerRepoPermission(ctx, project.Repo, doer)
		if err != nil {
			return false, err
		}
		return perm.CanWrite(unit.TypeIssues) && perm.CanWrite(unit.TypePullRequests), nil
	}
	if !user_model.DoerCanViewPrivate(doer) { // owner-level projects hold items of the owner's private repositories too
		return false, nil
	}
	if project.Type == project_model.TypeOrganization {
		if user_model.DoerIsSiteAdminFullAccess(doer) {
			return true, nil
		}
		return organization.IsOrganizationOwner(ctx, project.OwnerID, doer.ID)
	}
	return doer.ID == project.OwnerID, nil
}

func checkCanManageWorkflows(ctx context.Context, doer *user_model.User, project *project_model.Project) error {
	allowed, err := CanManageWorkflows(ctx, doer, project)
	if err != nil {
		return err
	}
	if !allowed {
		return util.NewPermissionDeniedErrorf("not allowed to manage the workflows of this project")
	}
	return nil
}

func CreateWorkflow(ctx context.Context, doer *user_model.User, project *project_model.Project, opts *api.CreateProjectWorkflowOption) (*project_model.Workflow, error) {
	if err := checkCanManageWorkflows(ctx, doer, project); err != nil {
		return nil, err
	}
	wf := &project_model.Workflow{
		ProjectID:     project.ID,
		WorkflowEvent: project_model.WorkflowEvent(opts.Event),
		Enabled:       optional.FromPtr(opts.Enabled).ValueOrDefault(true),
	}
	if !wf.WorkflowEvent.IsValid() {
		return nil, util.ErrorWrap(util.ErrUnprocessableContent, "invalid event: %s", opts.Event)
	}
	if err := setWorkflowRules(ctx, project, wf, &opts.Filters, &opts.Actions); err != nil {
		return nil, err
	}
	if err := project_model.CreateWorkflow(ctx, wf); err != nil {
		return nil, err
	}
	recordWorkflowAudit(ctx, doer, audit_model.ProjectWorkflowCreate, project, wf)
	return wf, nil
}

func UpdateWorkflow(ctx context.Context, doer *user_model.User, project *project_model.Project, wf *project_model.Workflow, opts *api.EditProjectWorkflowOption) error {
	if err := checkCanManageWorkflows(ctx, doer, project); err != nil {
		return err
	}
	var cols []string
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
	if len(cols) == 0 {
		return nil
	}
	if err := project_model.UpdateWorkflow(ctx, wf, cols...); err != nil {
		return err
	}
	recordWorkflowAudit(ctx, doer, audit_model.ProjectWorkflowUpdate, project, wf)
	return nil
}

func DeleteWorkflow(ctx context.Context, doer *user_model.User, project *project_model.Project, wf *project_model.Workflow) error {
	if err := checkCanManageWorkflows(ctx, doer, project); err != nil {
		return err
	}
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
	if len(actions.Types()) == 0 {
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
