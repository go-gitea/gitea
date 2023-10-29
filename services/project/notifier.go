// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/references"
	notify_service "code.gitea.io/gitea/services/notify"
)

type projectNotifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = &projectNotifier{}

func NewNotifier() notify_service.Notifier {
	return &projectNotifier{}
}

// Closed, Reopened
func (n *projectNotifier) IssueChangeStatus(ctx context.Context, doer *user_model.User, commitID string, issue *issues_model.Issue, _ *issues_model.Comment, isClosed bool) {
	triggerType := project_model.AutomationTriggerTypeStatus
	triggerData := int64(0)
	if isClosed {
		triggerData = 1
	}
	actions, err := project_model.FindAutomationsForTrigger(ctx, issue.ID, triggerType, triggerData)
	if err != nil {
		log.Error("FindAutomationsForTrigger: %v", err)
		return
	}
	if len(actions) > 0 {
		dispatchActions(ctx, actions, issue, doer)
	}
}

// LabelAdded
func (n *projectNotifier) IssueChangeLabels(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, addedLabels, removedLabels []*issues_model.Label) {
	actions := make([]project_model.AutomationAction, 0, 10)
	for _, label := range addedLabels {
		triggerType := project_model.AutomationTriggerTypeLabel
		newActions, err := project_model.FindAutomationsForTrigger(ctx, issue.ID, triggerType, label.ID)
		if err == nil {
			actions = append(actions, newActions...)
		}
	}
	for _, label := range removedLabels {
		triggerType := project_model.AutomationTriggerTypeUnlabel
		newActions, err := project_model.FindAutomationsForTrigger(ctx, issue.ID, triggerType, label.ID)
		if err == nil {
			actions = append(actions, newActions...)
		}
	}
	if len(actions) > 0 {
		dispatchActions(ctx, actions, issue, doer)
	}
}

// ReviewerAssigned, ReviewersUnassigned
func (n *projectNotifier) PullRequestReviewRequest(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, reviewer *user_model.User, isAdd bool, comment *issues_model.Comment) {
	if pr, _ := issue.GetPullRequest(ctx); pr != nil {
		triggerType := project_model.AutomationTriggerTypeAssignReviewer
		if isAdd {
			// We want to trigger when the number of reviewers goes from 0 to 1
			if len(pr.RequestedReviewers) != 1 {
				return
			}
		} else {
			// We want to trigger when the number of reviewers goes to 0
			if len(pr.RequestedReviewers) != 0 {
				return
			}
			triggerType = project_model.AutomationTriggerTypeUnassignReviewers
		}

		actions, err := project_model.FindAutomationsForTrigger(ctx, issue.ID, triggerType, 0)
		if err != nil {
			log.Error("FindAutomationsForTrigger: %v", err)
			return
		}
		if len(actions) > 0 {
			dispatchActions(ctx, actions, issue, doer)
		}
	}
}

// Approve
func (n *projectNotifier) PullRequestReview(ctx context.Context, pr *issues_model.PullRequest, review *issues_model.Review, comment *issues_model.Comment, mentions []*user_model.User) {
	if review.Type == issues_model.ReviewTypeApprove {
		triggerType := project_model.AutomationTriggerTypeApprove
		if actions, err := project_model.FindAutomationsForTrigger(ctx, pr.IssueID, triggerType, 0); err == nil {
			if err := review.LoadReviewer(ctx); err != nil {
				log.Error("LoadReviewer: %v", err)
			}
			if err := pr.LoadIssue(ctx); err != nil {
				log.Error("LoadIssue: %v", err)
			}
			// Trigger actions for both target PR and cross referenced issues which will be closed
			dispatchActions(ctx, actions, pr.Issue, review.Reviewer)
			if comments, err := pr.ResolveCrossReferences(ctx); err == nil {
				for _, comment := range comments {
					if comment.RefAction == references.XRefActionCloses {
						if actions, err := project_model.FindAutomationsForTrigger(ctx, comment.IssueID, triggerType, 0); err == nil {
							if err := comment.LoadIssue(ctx); err != nil {
								log.Error("LoadIssue: %v", err)
							}
							dispatchActions(ctx, actions, comment.Issue, review.Reviewer)
						}
					}
				}
			}
		}
	}
}

// XRef
func (n *projectNotifier) NewPullRequest(ctx context.Context, pr *issues_model.PullRequest, mentions []*user_model.User) {
	triggerType := project_model.AutomationTriggerTypeXRef
	if comments, err := pr.ResolveCrossReferences(ctx); err == nil {
		for _, comment := range comments {
			if actions, err := project_model.FindAutomationsForTrigger(ctx, comment.IssueID, triggerType, int64(comment.RefAction)); err == nil {
				if err := comment.LoadIssue(ctx); err != nil {
					log.Error("LoadIssue: %v", err)
				}
				// Trigger actions for both target issue and source PR
				dispatchActions(ctx, actions, comment.Issue, pr.Issue.Poster)
				dispatchActions(ctx, actions, pr.Issue, pr.Issue.Poster)
			}
		}
	}
}
