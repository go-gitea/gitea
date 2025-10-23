// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package projects

import (
	"context"
	"slices"
	"strconv"
	"strings"

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

func fireIssueWorkflow(ctx context.Context, workflow *project_model.Workflow, issue *issues_model.Issue) {
	for _, filter := range workflow.WorkflowFilters {
		switch filter.Type {
		case project_model.WorkflowFilterTypeIssueType:
			values := strings.Split(filter.Value, ",")
			if !(slices.Contains(values, "issue") && !issue.IsPull) || (slices.Contains(values, "pull") && issue.IsPull) {
				return
			}
		default:
			log.Error("Unsupported filter type: %s", filter.Type)
			return
		}
	}

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
			// TODO: implement adding labels
		case project_model.WorkflowActionTypeRemoveLabels:
			// TODO: implement removing labels
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
