// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	user_model "code.gitea.io/gitea/models/user"
)

// MoveIssuesOnProjectColumn moves or keeps issues in a column and sorts them inside that column
func MoveIssuesOnProjectColumn(ctx context.Context, doer *user_model.User, column *project_model.Column, sortedIssueIDs map[int64]int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		issueIDs := make([]int64, 0, len(sortedIssueIDs))
		for _, issueID := range sortedIssueIDs {
			issueIDs = append(issueIDs, issueID)
		}
		count, err := db.GetEngine(ctx).
			Where("project_id=?", column.ProjectID).
			In("issue_id", issueIDs).
			Count(new(project_model.ProjectIssue))
		if err != nil {
			return err
		}
		if int(count) != len(sortedIssueIDs) {
			return fmt.Errorf("all issues have to be added to a project first")
		}

		issues, err := issues_model.GetIssuesByIDs(ctx, issueIDs)
		if err != nil {
			return err
		}
		if _, err := issues.LoadRepositories(ctx); err != nil {
			return err
		}

		project, err := project_model.GetProjectByID(ctx, column.ProjectID)
		if err != nil {
			return err
		}

		issuesMap := make(map[int64]*issues_model.Issue, len(issues))
		for _, issue := range issues {
			issuesMap[issue.ID] = issue
		}

		for sorting, issueID := range sortedIssueIDs {
			curIssue := issuesMap[issueID]
			if curIssue == nil {
				continue
			}

			_, err = db.Exec(ctx, "UPDATE `project_issue` SET project_board_id=?, sorting=? WHERE issue_id=?", column.ID, sorting, issueID)
			if err != nil {
				return err
			}

			// add timeline to issue
			if _, err := issues_model.CreateComment(ctx, &issues_model.CreateCommentOptions{
				Type:               issues_model.CommentTypeProjectColumn,
				Doer:               doer,
				Repo:               curIssue.Repo,
				Issue:              curIssue,
				ProjectID:          column.ProjectID,
				ProjectTitle:       project.Title,
				ProjectColumnID:    column.ID,
				ProjectColumnTitle: column.Title,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}
