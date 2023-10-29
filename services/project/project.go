// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
)

func MoveIssuesOnProjectBoard(ctx context.Context, doer *user_model.User, board *project_model.Board, sortedIssueIDs map[int64]int64) (bool, error) {
	type MovedIssue struct {
		issue   *issues_model.Issue
		actions []project_model.AutomationAction
	}
	var movedIssues []MovedIssue

	if setting.ProjectAutomation.Enabled {
		movedIssues = make([]MovedIssue, 0, 10)
		triggerType := project_model.AutomationTriggerTypeMove
		for _, issueID := range sortedIssueIDs {
			issue, _ := issues_model.GetIssueByID(ctx, issueID)
			if issue != nil && !issue.IsOnProjectBoard(ctx, board) {
				actions, _ := project_model.FindAutomationsForTrigger(ctx, issue.ID, triggerType, board.ID)
				if len(actions) > 0 {
					movedIssues = append(movedIssues, MovedIssue{issue: issue, actions: actions})
				}
			}
		}
	}

	err := project_model.MoveIssuesOnProjectBoard(ctx, board, sortedIssueIDs)

	reloadHint := false
	if err == nil && len(movedIssues) > 0 {
		for _, movedIssue := range movedIssues {
			if dispatchActions(ctx, movedIssue.actions, movedIssue.issue, doer) {
				reloadHint = true
			}
		}
	}

	return reloadHint, err
}
