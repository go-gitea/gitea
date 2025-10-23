// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package projects

import (
	"context"
	"strconv"

	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
	issue_service "code.gitea.io/gitea/services/issue"
	notify_service "code.gitea.io/gitea/services/notify"
)

func init() {
	notify_service.RegisterNotifier(&workflowNotifier{})
}

type workflowNotifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = &workflowNotifier{}

// NewNotifier create a new workflowNotifier notifier
func NewNotifier() notify_service.Notifier {
	return &workflowNotifier{}
}

func (m *workflowNotifier) NewIssue(ctx context.Context, issue *issues_model.Issue, mentions []*user_model.User) {
	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("NewIssue: LoadRepo: %v", err)
		return
	}
	if err := issue.LoadProject(ctx); err != nil {
		log.Error("NewIssue: LoadProject: %v", err)
		return
	}
	if issue.Project == nil {
		// TODO: handle item opened
		return
	}

	workflows, err := project_model.FindWorkflowsByProjectID(ctx, issue.Project.ID)
	if err != nil {
		log.Error("NewIssue: FindWorkflowsByProjectID: %v", err)
		return
	}

	// Find workflows for the ItemOpened event
	for _, workflow := range workflows {
		if workflow.WorkflowEvent == project_model.WorkflowEventItemOpened {
			fireIssueWorkflow(ctx, workflow, issue)
		}
	}
}

func (m *workflowNotifier) NewPullRequest(ctx context.Context, pr *issues_model.PullRequest, mentions []*user_model.User) {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("NewIssue: LoadIssue: %v", err)
		return
	}
	issue := pr.Issue
	m.NewIssue(ctx, issue, mentions)
}

func (m *workflowNotifier) IssueChangeStatus(ctx context.Context, doer *user_model.User, commitID string, issue *issues_model.Issue, actionComment *issues_model.Comment, isClosed bool) {
	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("IssueChangeStatus: LoadRepo: %v", err)
		return
	}
	if err := issue.LoadProject(ctx); err != nil {
		log.Error("NewIssue: LoadProject: %v", err)
		return
	}
	if issue.Project == nil {
		return
	}

	workflows, err := project_model.FindWorkflowsByProjectID(ctx, issue.Project.ID)
	if err != nil {
		log.Error("IssueChangeStatus: FindWorkflowsByProjectID: %v", err)
		return
	}

	workflowEvent := util.Iif(isClosed, project_model.WorkflowEventItemClosed, project_model.WorkflowEventItemReopened)
	// Find workflows for the specific event
	for _, workflow := range workflows {
		if workflow.WorkflowEvent == workflowEvent {
			fireIssueWorkflow(ctx, workflow, issue)
		}
	}
}

func (*workflowNotifier) IssueChangeProjects(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, newProject *project_model.Project) {
	if newProject == nil {
		return
	}

	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("IssueChangeStatus: LoadRepo: %v", err)
		return
	}

	if err := issue.LoadProject(ctx); err != nil {
		log.Error("NewIssue: LoadProject: %v", err)
		return
	}
	if issue.Project == nil || issue.Project.ID != newProject.ID {
		return
	}

	workflows, err := project_model.FindWorkflowsByProjectID(ctx, issue.Project.ID)
	if err != nil {
		log.Error("IssueChangeStatus: FindWorkflowsByProjectID: %v", err)
		return
	}

	// Find workflows for the ItemOpened event
	for _, workflow := range workflows {
		if workflow.WorkflowEvent == project_model.WorkflowEventItemAddedToProject {
			fireIssueWorkflow(ctx, workflow, issue)
		}
	}
}

func (*workflowNotifier) IssueChangeProjectColumn(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, newColumnID int64) {
	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("IssueChangeStatus: LoadRepo: %v", err)
		return
	}

	if err := issue.LoadProject(ctx); err != nil {
		log.Error("NewIssue: LoadProject: %v", err)
		return
	}

	newColumn, err := project_model.GetColumn(ctx, newColumnID)
	if err != nil {
		log.Error("IssueChangeProjectColumn: GetColumn: %v", err)
		return
	}
	if issue.Project == nil || issue.Project.ID != newColumn.ProjectID {
		return
	}

	workflows, err := project_model.FindWorkflowsByProjectID(ctx, issue.Project.ID)
	if err != nil {
		log.Error("IssueChangeStatus: FindWorkflowsByProjectID: %v", err)
		return
	}

	// Find workflows for the ItemColumnChanged event
	for _, workflow := range workflows {
		if workflow.WorkflowEvent == project_model.WorkflowEventItemColumnChanged {
			fireIssueWorkflowWithColumn(ctx, workflow, issue, newColumnID)
		}
	}
}

func (*workflowNotifier) MergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("NewIssue: LoadIssue: %v", err)
		return
	}
	issue := pr.Issue

	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("IssueChangeStatus: LoadRepo: %v", err)
		return
	}

	if err := issue.LoadProject(ctx); err != nil {
		log.Error("NewIssue: LoadProject: %v", err)
		return
	}
	if issue.Project == nil {
		return
	}

	workflows, err := project_model.FindWorkflowsByProjectID(ctx, issue.Project.ID)
	if err != nil {
		log.Error("IssueChangeStatus: FindWorkflowsByProjectID: %v", err)
		return
	}

	// Find workflows for the PullRequestMerged event
	for _, workflow := range workflows {
		if workflow.WorkflowEvent == project_model.WorkflowEventPullRequestMerged {
			fireIssueWorkflow(ctx, workflow, issue)
		}
	}
}

func (m *workflowNotifier) AutoMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	m.MergePullRequest(ctx, doer, pr)
}

func (*workflowNotifier) PullRequestReview(ctx context.Context, pr *issues_model.PullRequest, review *issues_model.Review, comment *issues_model.Comment, mentions []*user_model.User) {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("NewIssue: LoadIssue: %v", err)
		return
	}
	issue := pr.Issue

	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("IssueChangeStatus: LoadRepo: %v", err)
		return
	}

	if err := issue.LoadProject(ctx); err != nil {
		log.Error("NewIssue: LoadProject: %v", err)
		return
	}
	if issue.Project == nil {
		return
	}

	workflows, err := project_model.FindWorkflowsByProjectID(ctx, issue.Project.ID)
	if err != nil {
		log.Error("IssueChangeStatus: FindWorkflowsByProjectID: %v", err)
		return
	}

	// Find workflows for the PullRequestMerged event
	for _, workflow := range workflows {
		if (workflow.WorkflowEvent == project_model.WorkflowEventCodeChangesRequested && review.Type == issues_model.ReviewTypeReject) ||
			(workflow.WorkflowEvent == project_model.WorkflowEventCodeReviewApproved && review.Type == issues_model.ReviewTypeApprove) {
			fireIssueWorkflow(ctx, workflow, issue)
		}
	}
}

// fireIssueWorkflowWithColumn fires a workflow for an issue with a specific column ID
// This is used for ItemColumnChanged events where we need to check the target column
func fireIssueWorkflowWithColumn(ctx context.Context, workflow *project_model.Workflow, issue *issues_model.Issue, columnID int64) {
	// Load issue labels for labels filter
	if err := issue.LoadLabels(ctx); err != nil {
		log.Error("LoadLabels: %v", err)
		return
	}

	for _, filter := range workflow.WorkflowFilters {
		switch filter.Type {
		case project_model.WorkflowFilterTypeIssueType:
			// If filter value is empty, match all types
			if filter.Value == "" {
				continue
			}
			// Filter value can be "issue" or "pull_request"
			if filter.Value == "issue" && issue.IsPull {
				return
			}
			if filter.Value == "pull_request" && !issue.IsPull {
				return
			}
		case project_model.WorkflowFilterTypeColumn:
			// If filter value is empty, match all columns
			if filter.Value == "" {
				continue
			}
			filterColumnID, _ := strconv.ParseInt(filter.Value, 10, 64)
			if filterColumnID == 0 {
				log.Error("Invalid column ID: %s", filter.Value)
				return
			}
			// For column changed event, check against the new column ID
			if columnID != filterColumnID {
				return
			}
		case project_model.WorkflowFilterTypeLabels:
			// Check if issue has the specified label
			labelID, _ := strconv.ParseInt(filter.Value, 10, 64)
			if labelID == 0 {
				log.Error("Invalid label ID: %s", filter.Value)
				return
			}
			// Check if issue has this label
			hasLabel := false
			for _, label := range issue.Labels {
				if label.ID == labelID {
					hasLabel = true
					break
				}
			}
			if !hasLabel {
				return
			}
		default:
			log.Error("Unsupported filter type: %s", filter.Type)
			return
		}
	}

	executeWorkflowActions(ctx, workflow, issue)
}

func fireIssueWorkflow(ctx context.Context, workflow *project_model.Workflow, issue *issues_model.Issue) {
	// Load issue labels for labels filter
	if err := issue.LoadLabels(ctx); err != nil {
		log.Error("LoadLabels: %v", err)
		return
	}

	for _, filter := range workflow.WorkflowFilters {
		switch filter.Type {
		case project_model.WorkflowFilterTypeIssueType:
			// If filter value is empty, match all types
			if filter.Value == "" {
				continue
			}
			// Filter value can be "issue" or "pull_request"
			if filter.Value == "issue" && issue.IsPull {
				return
			}
			if filter.Value == "pull_request" && !issue.IsPull {
				return
			}
		case project_model.WorkflowFilterTypeColumn:
			// If filter value is empty, match all columns
			if filter.Value == "" {
				continue
			}
			columnID, _ := strconv.ParseInt(filter.Value, 10, 64)
			if columnID == 0 {
				log.Error("Invalid column ID: %s", filter.Value)
				return
			}
			issueProjectColumnID, err := issue.ProjectColumnID(ctx)
			if err != nil {
				log.Error("Issue.ProjectColumnID: %v", err)
				return
			}
			if issueProjectColumnID != columnID {
				return
			}
		case project_model.WorkflowFilterTypeLabels:
			// Check if issue has the specified label
			labelID, _ := strconv.ParseInt(filter.Value, 10, 64)
			if labelID == 0 {
				log.Error("Invalid label ID: %s", filter.Value)
				return
			}
			// Check if issue has this label
			hasLabel := false
			for _, label := range issue.Labels {
				if label.ID == labelID {
					hasLabel = true
					break
				}
			}
			if !hasLabel {
				return
			}
		default:
			log.Error("Unsupported filter type: %s", filter.Type)
			return
		}
	}

	executeWorkflowActions(ctx, workflow, issue)
}

func executeWorkflowActions(ctx context.Context, workflow *project_model.Workflow, issue *issues_model.Issue) {
	for _, action := range workflow.WorkflowActions {
		switch action.Type {
		case project_model.WorkflowActionTypeColumn:
			columnID, _ := strconv.ParseInt(action.Value, 10, 64)
			if columnID == 0 {
				log.Error("Invalid column ID: %s", action.Value)
				continue
			}
			column, err := project_model.GetColumnByProjectIDAndColumnID(ctx, issue.Project.ID, columnID)
			if err != nil {
				log.Error("GetColumnByProjectIDAndColumnID: %v", err)
				continue
			}
			if err := MoveIssueToAnotherColumn(ctx, user_model.NewProjectWorkflowsUser(), issue, column); err != nil {
				log.Error("MoveIssueToAnotherColumn: %v", err)
				continue
			}
		case project_model.WorkflowActionTypeAddLabels:
			labelID, _ := strconv.ParseInt(action.Value, 10, 64)
			if labelID == 0 {
				log.Error("Invalid label ID: %s", action.Value)
				continue
			}
			label, err := issues_model.GetLabelByID(ctx, labelID)
			if err != nil {
				log.Error("GetLabelByID: %v", err)
				continue
			}
			if err := issue_service.AddLabel(ctx, issue, user_model.NewProjectWorkflowsUser(), label); err != nil {
				log.Error("AddLabels: %v", err)
				continue
			}
		case project_model.WorkflowActionTypeRemoveLabels:
			labelID, _ := strconv.ParseInt(action.Value, 10, 64)
			if labelID == 0 {
				log.Error("Invalid label ID: %s", action.Value)
				continue
			}
			label, err := issues_model.GetLabelByID(ctx, labelID)
			if err != nil {
				log.Error("GetLabelByID: %v", err)
				continue
			}
			if err := issue_service.RemoveLabel(ctx, issue, user_model.NewProjectWorkflowsUser(), label); err != nil {
				if !issues_model.IsErrRepoLabelNotExist(err) {
					log.Error("RemoveLabels: %v", err)
				}
				continue
			}
		case project_model.WorkflowActionTypeClose:
			if err := issue_service.CloseIssue(ctx, issue, user_model.NewProjectWorkflowsUser(), ""); err != nil {
				log.Error("CloseIssue: %v", err)
				continue
			}
		default:
			log.Error("Unsupported action type: %s", action.Type)
		}
	}
}
