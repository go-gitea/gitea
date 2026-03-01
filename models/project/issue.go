// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"errors"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/util"
)

// ProjectIssue saves relation from issue to a project
type ProjectIssue struct { //revive:disable-line:exported
	ID        int64 `xorm:"pk autoincr"`
	IssueID   int64 `xorm:"INDEX"`
	ProjectID int64 `xorm:"INDEX"`

	// ProjectColumnID should not be zero since 1.22. If it's zero, the issue will not be displayed on UI and it might result in errors.
	ProjectColumnID int64 `xorm:"'project_board_id' INDEX"`

	// the sorting order on the column
	Sorting int64 `xorm:"NOT NULL DEFAULT 0"`
}

func init() {
	db.RegisterModel(new(ProjectIssue))
}

func deleteProjectIssuesByProjectID(ctx context.Context, projectID int64) error {
	_, err := db.GetEngine(ctx).Where("project_id=?", projectID).Delete(&ProjectIssue{})
	return err
}

func (c *Column) moveIssuesToAnotherColumn(ctx context.Context, newColumn *Column) error {
	if c.ProjectID != newColumn.ProjectID {
		return errors.New("columns have to be in the same project")
	}

	if c.ID == newColumn.ID {
		return nil
	}

	res := struct {
		MaxSorting int64
		IssueCount int64
	}{}
	if _, err := db.GetEngine(ctx).Select("max(sorting) as max_sorting, count(*) as issue_count").
		Table("project_issue").
		Where("project_id=?", newColumn.ProjectID).
		And("project_board_id=?", newColumn.ID).
		Get(&res); err != nil {
		return err
	}

	issues, err := c.GetIssues(ctx)
	if err != nil {
		return err
	}
	if len(issues) == 0 {
		return nil
	}

	nextSorting := util.Iif(res.IssueCount > 0, res.MaxSorting+1, 0)
	return db.WithTx(ctx, func(ctx context.Context) error {
		for i, issue := range issues {
			issue.ProjectColumnID = newColumn.ID
			issue.Sorting = nextSorting + int64(i)
			if _, err := db.GetEngine(ctx).ID(issue.ID).Cols("project_board_id", "sorting").Update(issue); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteAllProjectIssueByIssueIDsAndProjectIDs delete all project's issues by issue's and project's ids
func DeleteAllProjectIssueByIssueIDsAndProjectIDs(ctx context.Context, issueIDs, projectIDs []int64) error {
	_, err := db.GetEngine(ctx).In("project_id", projectIDs).In("issue_id", issueIDs).Delete(&ProjectIssue{})
	return err
}

// AddOrUpdateIssueToColumn adds an issue to a project column or moves an existing one
func AddOrUpdateIssueToColumn(ctx context.Context, issueID int64, column *Column) error {
	// Check if the issue is already in this project
	existingPI := &ProjectIssue{}
	has, err := db.GetEngine(ctx).Where("project_id=? AND issue_id=?", column.ProjectID, issueID).Get(existingPI)
	if err != nil {
		return err
	}

	// If already exists, just update the column
	if has {
		if existingPI.ProjectColumnID == column.ID {
			// Already in this column, nothing to do
			return nil
		}
		// Move to new column - need to update sorting
		res := struct {
			MaxSorting int64
			IssueCount int64
		}{}
		if _, err := db.GetEngine(ctx).Select("max(sorting) as max_sorting, count(*) as issue_count").
			Table("project_issue").
			Where("project_id=?", column.ProjectID).
			And("project_board_id=?", column.ID).
			Get(&res); err != nil {
			return err
		}

		nextSorting := util.Iif(res.IssueCount > 0, res.MaxSorting+1, 0)
		existingPI.ProjectColumnID = column.ID
		existingPI.Sorting = nextSorting
		_, err = db.GetEngine(ctx).ID(existingPI.ID).Cols("project_board_id", "sorting").Update(existingPI)
		return err
	}

	// Calculate next sorting value
	res := struct {
		MaxSorting int64
		IssueCount int64
	}{}
	if _, err := db.GetEngine(ctx).Select("max(sorting) as max_sorting, count(*) as issue_count").
		Table("project_issue").
		Where("project_id=?", column.ProjectID).
		And("project_board_id=?", column.ID).
		Get(&res); err != nil {
		return err
	}

	nextSorting := util.Iif(res.IssueCount > 0, res.MaxSorting+1, 0)

	// Create new ProjectIssue
	pi := &ProjectIssue{
		IssueID:         issueID,
		ProjectID:       column.ProjectID,
		ProjectColumnID: column.ID,
		Sorting:         nextSorting,
	}

	return db.Insert(ctx, pi)
}
