// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"

	issues_model "gitea.dev/models/issues"
	project_model "gitea.dev/models/project"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/util"
	notify_service "gitea.dev/services/notify"
)

func init() {
	notify_service.RegisterNotifier(&workflowNotifier{})
}

// workflowNotifier moves project cards according to the project workflows
type workflowNotifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = &workflowNotifier{}

func (n *workflowNotifier) IssueChangeStatus(ctx context.Context, doer *user_model.User, _ string, issue *issues_model.Issue, _ *issues_model.Comment, isClosed bool) {
	if issue.IsPull {
		if err := issue.LoadPullRequest(ctx); err != nil {
			log.Error("LoadPullRequest: %v", err)
			return
		}
		if issue.PullRequest.HasMerged {
			return // handled by the "pull request merged" workflow
		}
	}
	ApplyWorkflows(ctx, doer, issue, util.Iif(isClosed, project_model.WorkflowEventItemClosed, project_model.WorkflowEventItemReopened))
}

func (n *workflowNotifier) MergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue: %v", err)
		return
	}
	ApplyWorkflows(ctx, doer, pr.Issue, project_model.WorkflowEventPullRequestMerged)
}

func (n *workflowNotifier) AutoMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	n.MergePullRequest(ctx, doer, pr)
}

func (n *workflowNotifier) PullRequestReview(ctx context.Context, pr *issues_model.PullRequest, review *issues_model.Review, _ *issues_model.Comment, _ []*user_model.User) {
	var event project_model.WorkflowEvent
	switch review.Type {
	case issues_model.ReviewTypeApprove:
		event = project_model.WorkflowEventReviewApproved
	case issues_model.ReviewTypeReject:
		event = project_model.WorkflowEventChangesRequested
	default:
		return
	}
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue: %v", err)
		return
	}
	if err := review.LoadReviewer(ctx); err != nil {
		log.Error("LoadReviewer: %v", err)
		return
	}
	ApplyWorkflows(ctx, review.Reviewer, pr.Issue, event)
}

// ApplyWorkflows moves the issue's card in every project that has an enabled workflow for the event.
// Failures are logged only, the change that raised the event has already happened.
func ApplyWorkflows(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, event project_model.WorkflowEvent) {
	projectIDs, err := issue.ProjectIDs(ctx)
	if err != nil {
		log.Error("ProjectIDs: %v", err)
		return
	}
	workflows, err := project_model.GetEnabledWorkflows(ctx, projectIDs, event)
	if err != nil {
		log.Error("GetEnabledWorkflows: %v", err)
		return
	}
	if len(workflows) == 0 {
		return
	}

	columnMap, err := issue.ProjectColumnMap(ctx)
	if err != nil {
		log.Error("ProjectColumnMap: %v", err)
		return
	}
	for projectID, w := range workflows {
		if columnMap[projectID] == w.ColumnID {
			continue
		}
		column, err := project_model.GetColumnByIDAndProjectID(ctx, w.ColumnID, projectID)
		if err != nil {
			log.Error("GetColumnByIDAndProjectID: %v", err)
			continue
		}
		if err := MoveIssueToColumn(ctx, doer, issue, column, optional.None[int64]()); err != nil {
			log.Error("MoveIssueToColumn: %v", err)
		}
	}
}
