// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"errors"

	"gitea.dev/models/db"
	"gitea.dev/modules/util"
)

// ProjectIssue saves relation from issue to a project
type ProjectIssue struct { //revive:disable-line:exported
	ID        int64 `xorm:"pk autoincr"`
	IssueID   int64 `xorm:"INDEX"`
	ProjectID int64 `xorm:"INDEX"`

	// ProjectColumnID should not be zero since 1.22. Legacy zero rows render in the default column.
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

// columnIssueIDs lists the project_board_id values a column claims. Rows written before
// 1.22 carry 0, which the board renders in the default column, so the default column has
// to claim them too.
func columnIssueIDs(column *Column) []int64 {
	if column.Default {
		return []int64{column.ID, 0}
	}
	return []int64{column.ID}
}

// IsIssueInColumn reports whether the issue is placed in the column.
func IsIssueInColumn(ctx context.Context, issueID int64, column *Column) (bool, error) {
	return db.GetEngine(ctx).
		Where("issue_id=?", issueID).
		And("project_id=?", column.ProjectID).
		In("project_board_id", columnIssueIDs(column)).
		Exist(new(ProjectIssue))
}

// GetColumnIssueIDs returns the IDs of the issues placed in a column.
func GetColumnIssueIDs(ctx context.Context, column *Column) ([]int64, error) {
	issueIDs := make([]int64, 0, 10)
	return issueIDs, db.GetEngine(ctx).Table("project_issue").
		Where("project_id=?", column.ProjectID).
		In("project_board_id", columnIssueIDs(column)).
		Cols("issue_id").Find(&issueIDs)
}

// GetColumnIssueNextSorting returns the sorting value to append an issue at the end of the column.
func GetColumnIssueNextSorting(ctx context.Context, column *Column) (int64, error) {
	res := struct {
		MaxSorting int64
		IssueCount int64
	}{}
	if _, err := db.GetEngine(ctx).Select("max(sorting) AS max_sorting, count(*) AS issue_count").
		Table("project_issue").
		Where("project_id=?", column.ProjectID).
		In("project_board_id", columnIssueIDs(column)).
		Get(&res); err != nil {
		return 0, err
	}
	return util.Iif(res.IssueCount > 0, res.MaxSorting+1, 0), nil
}

func moveIssuesToAnotherColumn(ctx context.Context, oldColumn, newColumn *Column) error {
	if oldColumn.ProjectID != newColumn.ProjectID {
		return errors.New("columns have to be in the same project")
	}

	if oldColumn.ID == newColumn.ID {
		return nil
	}

	movedIssues, err := oldColumn.GetIssues(ctx)
	if err != nil {
		return err
	}
	if len(movedIssues) == 0 {
		return nil
	}

	nextSorting, err := GetColumnIssueNextSorting(ctx, newColumn)
	if err != nil {
		return err
	}
	return db.WithTx(ctx, func(ctx context.Context) error {
		for i, issue := range movedIssues {
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
