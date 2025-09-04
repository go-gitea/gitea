// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package projects

import (
	"context"
	"slices"
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

	// Find workflows for the ItemAddedToProject event
	for _, workflow := range workflows {
		if workflow.WorkflowEvent == project_model.WorkflowEventItemAddedToProject {
			fireIssueWorkflow(ctx, workflow, issue)
		}
	}
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
		switch action.ActionType {
		case project_model.WorkflowActionTypeColumn:
			column, err := project_model.GetColumnByProjectIDAndColumnName(ctx, issue.Project.ID, action.ActionValue)
			if err != nil {
				log.Error("GetColumnByProjectIDAndColumnName: %v", err)
				continue
			}
			if err := project_model.AddIssueToColumn(ctx, issue.ID, column); err != nil {
				log.Error("AddIssueToColumn: %v", err)
				continue
			}
		case project_model.WorkflowActionTypeAddLabels:
		case project_model.WorkflowActionTypeRemoveLabels:
		case project_model.WorkflowActionTypeClose:
			if err := issue_service.CloseIssue(ctx, issue, user_model.NewWorkflowsUser(), ""); err != nil {
				log.Error("CloseIssue: %v", err)
				continue
			}
		default:
			log.Error("Unsupported action type: %s", action.ActionType)
		}
	}
}
