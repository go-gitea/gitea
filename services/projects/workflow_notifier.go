// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"slices"

	issues_model "gitea.dev/models/issues"
	project_model "gitea.dev/models/project"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/util"
	issue_service "gitea.dev/services/issue"
	notify_service "gitea.dev/services/notify"
)

func init() {
	notify_service.RegisterNotifier(&workflowNotifier{})
}

type workflowNotifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = &workflowNotifier{}

func (*workflowNotifier) NewIssue(ctx context.Context, issue *issues_model.Issue, _ []*user_model.User) {
	if err := issue.LoadPoster(ctx); err != nil {
		log.Error("LoadPoster: %v", err)
		return
	}
	runIssueProjectsWorkflows(ctx, issue.Poster, issue, project_model.WorkflowEventItemOpened)
}

func (m *workflowNotifier) NewPullRequest(ctx context.Context, pr *issues_model.PullRequest, mentions []*user_model.User) {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue: %v", err)
		return
	}
	m.NewIssue(ctx, pr.Issue, mentions)
}

func (*workflowNotifier) IssueChangeStatus(ctx context.Context, doer *user_model.User, _ string, issue *issues_model.Issue, _ *issues_model.Comment, isClosed bool) {
	runIssueProjectsWorkflows(ctx, doer, issue, util.Iif(isClosed, project_model.WorkflowEventItemClosed, project_model.WorkflowEventItemReopened))
}

func (*workflowNotifier) IssueChangeProjects(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, addedProjectIDs, removedProjectIDs []int64) {
	runWorkflows(ctx, doer, issue, removedProjectIDs, project_model.WorkflowEventItemRemovedFromProject, workflowTrigger{})
	runWorkflows(ctx, doer, issue, addedProjectIDs, project_model.WorkflowEventItemAddedToProject, workflowTrigger{})
}

func (*workflowNotifier) IssueChangeProjectColumn(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldColumnID int64, newColumn *project_model.Column) {
	runWorkflows(ctx, doer, issue, []int64{newColumn.ProjectID}, project_model.WorkflowEventItemColumnChanged, workflowTrigger{
		sourceColumnID: oldColumnID,
		targetColumnID: newColumn.ID,
	})
}

func (*workflowNotifier) MergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue: %v", err)
		return
	}
	runIssueProjectsWorkflows(ctx, doer, pr.Issue, project_model.WorkflowEventPullRequestMerged)
}

func (m *workflowNotifier) AutoMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	m.MergePullRequest(ctx, doer, pr)
}

func (*workflowNotifier) PullRequestReview(ctx context.Context, pr *issues_model.PullRequest, review *issues_model.Review, _ *issues_model.Comment, _ []*user_model.User) {
	if !review.Official {
		return
	}
	var event project_model.WorkflowEvent
	switch review.Type {
	case issues_model.ReviewTypeApprove:
		event = project_model.WorkflowEventCodeReviewApproved
	case issues_model.ReviewTypeReject:
		event = project_model.WorkflowEventCodeChangesRequested
	default:
		return
	}
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue: %v", err)
		return
	}
	runIssueProjectsWorkflows(ctx, review.Reviewer, pr.Issue, event)
}

type workflowTrigger struct {
	sourceColumnID int64
	targetColumnID int64
}

func runIssueProjectsWorkflows(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, event project_model.WorkflowEvent) {
	projectIDs, err := issue.ProjectIDs(ctx)
	if err != nil {
		log.Error("ProjectIDs: %v", err)
		return
	}
	runWorkflows(ctx, doer, issue, projectIDs, event, workflowTrigger{})
}

func runWorkflows(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, projectIDs []int64, event project_model.WorkflowEvent, trigger workflowTrigger) {
	if issues_model.IsProjectWorkflowContext(ctx) {
		return
	}
	workflows, err := project_model.FindEnabledWorkflows(ctx, projectIDs, event)
	if err != nil {
		log.Error("FindEnabledWorkflows: %v", err)
		return
	}
	if len(workflows) == 0 {
		return
	}
	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("LoadRepo: %v", err)
		return
	}
	if issue.Repo.IsArchived {
		return
	}
	if err := issue.LoadLabels(ctx); err != nil {
		log.Error("LoadLabels: %v", err)
		return
	}
	for _, wf := range workflows {
		if matchWorkflowFilters(wf, issue, trigger) {
			executeWorkflowActions(ctx, wf, doer, issue)
		}
	}
}

// matchWorkflowFilters fails closed: a filter that cannot be evaluated never matches
func matchWorkflowFilters(wf *project_model.Workflow, issue *issues_model.Issue, trigger workflowTrigger) bool {
	if wf.UnsupportedRule() != "" {
		return false
	}
	f := wf.WorkflowFilters
	if f.IssueType != "" && f.IssueType != util.Iif(issue.IsPull, project_model.WorkflowIssueTypePullRequest, project_model.WorkflowIssueTypeIssue) {
		return false
	}
	if f.SourceColumnID != 0 && f.SourceColumnID != trigger.sourceColumnID {
		return false
	}
	if f.TargetColumnID != 0 && f.TargetColumnID != trigger.targetColumnID {
		return false
	}
	for _, labelID := range f.LabelIDs {
		if !slices.ContainsFunc(issue.Labels, func(l *issues_model.Label) bool { return l.ID == labelID }) {
			return false
		}
	}
	return true
}

func executeWorkflowActions(ctx context.Context, wf *project_model.Workflow, doer *user_model.User, issue *issues_model.Issue) {
	ctx = issues_model.WithProjectWorkflow(ctx, wf.WorkflowEvent)
	a := wf.WorkflowActions

	if a.ChangesIssue() {
		allowed, err := canWorkflowChangeIssue(ctx, wf, issue)
		if err != nil {
			log.Error("canWorkflowChangeIssue: %v", err)
			return
		}
		if !allowed {
			log.Warn("Project workflow %d may not change issue %d, skipping its actions", wf.ID, issue.ID)
			return
		}
	}

	if a.ColumnID != 0 {
		if err := moveIssueByWorkflow(ctx, wf, doer, issue); err != nil {
			log.Error("Project workflow %d: move issue %d: %v", wf.ID, issue.ID, err)
		}
	}

	if len(a.AddLabelIDs) > 0 || len(a.RemoveLabelIDs) > 0 {
		if err := changeLabelsByWorkflow(ctx, doer, issue, a.AddLabelIDs, a.RemoveLabelIDs); err != nil {
			log.Error("Project workflow %d: change labels of issue %d: %v", wf.ID, issue.ID, err)
		}
	}

	var err error
	if a.IssueState == project_model.WorkflowIssueStateClose && !issue.IsClosed {
		err = issue_service.CloseIssue(ctx, issue, doer, "")
	} else if a.IssueState == project_model.WorkflowIssueStateReopen && issue.IsClosed {
		err = issue_service.ReopenIssue(ctx, issue, doer, "")
	}
	if err != nil {
		log.Error("Project workflow %d: change state of issue %d: %v", wf.ID, issue.ID, err)
	}
}

func moveIssueByWorkflow(ctx context.Context, wf *project_model.Workflow, doer *user_model.User, issue *issues_model.Issue) error {
	column, err := project_model.GetColumnByIDAndProjectID(ctx, wf.WorkflowActions.ColumnID, wf.ProjectID)
	if project_model.IsErrProjectColumnNotExist(err) {
		log.Warn("Project workflow %d: column %d no longer exists", wf.ID, wf.WorkflowActions.ColumnID)
		return nil
	} else if err != nil {
		return err
	}
	columnIDs, err := issue.ProjectColumnMap(ctx)
	if err != nil {
		return err
	}
	if columnIDs[wf.ProjectID] == column.ID {
		return nil
	}
	return MoveIssueToColumn(ctx, doer, issue, column, optional.None[int64]())
}

func changeLabelsByWorkflow(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, addLabelIDs, removeLabelIDs []int64) error {
	labels, err := issues_model.GetLabelsByIDs(ctx, slices.Concat(addLabelIDs, removeLabelIDs))
	if err != nil {
		return err
	}
	addLabels := slices.DeleteFunc(slices.Clone(labels), func(l *issues_model.Label) bool { return !slices.Contains(addLabelIDs, l.ID) })
	removeLabels := slices.DeleteFunc(labels, func(l *issues_model.Label) bool { return !slices.Contains(removeLabelIDs, l.ID) })
	return issue_service.AddRemoveLabels(ctx, issue, doer, addLabels, removeLabels)
}
