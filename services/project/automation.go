// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"slices"

	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	issue_service "code.gitea.io/gitea/services/issue"
	notify_service "code.gitea.io/gitea/services/notify"
)

type ActionsQueueEntry struct {
	action project_model.AutomationAction
	issue  *issues_model.Issue
}

var (
	actionsQueue        []ActionsQueueEntry
	dispatchedActionIDs []int64
)

func Init() error {
	actionsQueue = nil
	dispatchedActionIDs = nil
	if setting.ProjectAutomation.Enabled {
		notify_service.RegisterNotifier(NewNotifier())
	}
	return nil
}

func dispatchActions(ctx context.Context, actions []project_model.AutomationAction, issue *issues_model.Issue, doer *user_model.User) bool {
	// the "root dispatch" is responsible for handling the actions queue
	isRootDispatch := actionsQueue == nil
	if isRootDispatch {
		actionsQueue = make([]ActionsQueueEntry, 0, 10)
		dispatchedActionIDs = make([]int64, 0, 10)
	}

	// append actions to queue (prevent running the same action twice)
	for _, action := range actions {
		actionID := action.Automation.ID
		if slices.Contains(dispatchedActionIDs, action.Automation.ID) {
			log.Warn("Possible loop relating to project automation ID %d while processing issue %d",
				action.Automation.ID, issue.ID)
			continue
		}
		dispatchedActionIDs = append(dispatchedActionIDs, actionID)
		actionsQueue = append(actionsQueue, ActionsQueueEntry{action, issue})
	}

	// only the "root dispatch" should process the queue
	if !isRootDispatch {
		return false
	}

	reloadHint := false
	for {
		if len(actionsQueue) == 0 {
			break
		}
		entry := actionsQueue[0]
		actionsQueue = actionsQueue[1:]
		action := entry.action
		issue := entry.issue

		target := project_model.AutomationActionTargetTypeIssue
		if issue.IsPull {
			target = project_model.AutomationActionTargetTypePullRequest
		}

		if !action.Automation.ShouldRunForTarget(target) {
			continue
		}

		switch action.Type {
		// Label - add label to issue
		case project_model.AutomationActionTypeLabel:
			if addLabel, _ := issues_model.GetLabelByID(ctx, action.Data); addLabel != nil {
				if err := issue.LoadLabels(ctx); err != nil {
					log.Error("LoadLabels: %v", err)
				}
				if !slices.ContainsFunc(issue.Labels, func(label *issues_model.Label) bool { return label.ID == addLabel.ID }) {
					reloadHint = true
					if err := issue_service.AddLabel(ctx, issue, doer, addLabel); err != nil {
						log.Error("AddLabel: %v", err)
					}
				}

			}

		// Unlabel - remove label from issue
		case project_model.AutomationActionTypeUnlabel:
			if removeLabel, _ := issues_model.GetLabelByID(ctx, action.Data); removeLabel != nil {
				if err := issue.LoadLabels(ctx); err != nil {
					log.Error("LoadLabels: %v", err)
				}
				if slices.ContainsFunc(issue.Labels, func(label *issues_model.Label) bool { return label.ID == removeLabel.ID }) {
					reloadHint = true
					if err := issue_service.RemoveLabel(ctx, issue, doer, removeLabel); err != nil {
						log.Error("RemoveLabel: %v", err)
					}
				}
			}

		// Move - move issue to column
		case project_model.AutomationActionTypeMove:
			if board, _ := project_model.GetBoard(ctx, action.Data); board != nil {
				if hint, _ := MoveIssuesOnProjectBoard(ctx, doer, board, map[int64]int64{0: issue.ID}); hint {
					reloadHint = true
				}
			}

		// Status - change status of issue / pr
		case project_model.AutomationActionTypeStatus:
			isClosed := action.Data > 0
			if isClosed != issue.IsClosed {
				reloadHint = true
				if err := issue_service.ChangeStatus(ctx, issue, doer, "", isClosed); err != nil {
					log.Error("ChangeStatus: %v", err)
				}
			}

		// Assign - assign issue / pr to current user
		case project_model.AutomationActionTypeAssign:
			if len(issue.Assignees) == 0 {
				if err := issue.LoadRepo(ctx); err == nil {
					reloadHint = true
					if _, err := issue_service.AddAssigneeIfNotAssigned(ctx, issue, doer, doer.ID, true); err != nil {
						log.Error("AddAssigneeIfNotAssigned: %v", err)
					}
				}
			}

		// Unassign - remove all assignees from issue / pr
		case project_model.AutomationActionTypeUnassign:
			if err := issue.LoadAssignees(ctx); err != nil {
				continue
			}
			if len(issue.Assignees) > 0 {
				if err := issue.LoadRepo(ctx); err == nil {
					reloadHint = true
					if err := issue_service.DeleteNotPassedAssignee(ctx, issue, doer, []*user_model.User{}); err != nil {
						log.Error("DeleteNotPassedAssignee: %v", err)
					}
				}
			}

		// AssignReviewer - add reviewer to pr
		case project_model.AutomationActionTypeAssignReviewer:
			if pr, _ := issue.GetPullRequest(ctx); pr != nil {
				if len(pr.RequestedReviewers) == 0 {
					reloadHint = true
					if _, err := issue_service.ReviewRequest(ctx, issue, doer, doer, true); err != nil {
						log.Error("ReviewRequest: %v", err)
					}
				}
			}

		// UnassignReviewers - remove all reviewers from pr
		case project_model.AutomationActionTypeUnassignReviewers:
			if pr, _ := issue.GetPullRequest(ctx); pr != nil {
				for _, reviewer := range pr.RequestedReviewers {
					reloadHint = true
					if _, err := issue_service.ReviewRequest(ctx, issue, doer, reviewer, false); err != nil {
						log.Error("ReviewRequest: %v", err)
					}
				}
			}

		// AssignProject - add issue / pr to current project
		case project_model.AutomationActionTypeAssignProject:
			if err := issue.LoadProject(ctx); err != nil {
				log.Error("LoadProject: %v", err)
			}
			if issue.Project == nil {
				reloadHint = true
				if err := issues_model.ChangeProjectAssign(ctx, issue, doer, action.Automation.ProjectID); err != nil {
					log.Error("ChangeProjectAssign: %v", err)
				}
			}

		// Approve - approve pull request
		case project_model.AutomationActionTypeApprove:
			// can not approve/reject your own PR
			if issue.PosterID != doer.ID {
				// Don't approve if you already approved it
				review, err := issues_model.GetReviewByIssueIDAndUserID(ctx, issue.ID, doer.ID)
				if issues_model.IsErrReviewNotExist(err) || (err == nil && review != nil && review.Type != issues_model.ReviewTypeApprove) {
					if pr, _ := issue.GetPullRequest(ctx); pr != nil {
						reloadHint = true
						if err := issue.LoadRepo(ctx); err != nil {
							log.Error("LoadRepo: %v", err)
						}
						if _, _, err := issues_model.SubmitReview(ctx, doer, issue, issues_model.ReviewTypeApprove, "", pr.HeadCommitID, false, []string{}); err != nil {
							log.Error("SubmitReview: %v", err)
						}
					}
				}
			}

		// NoOperation
		case project_model.AutomationActionTypeNoOperation:
			// no operation
		}
	}

	actionsQueue = nil
	dispatchedActionIDs = nil

	return reloadHint
}
