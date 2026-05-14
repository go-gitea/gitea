// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/xorm/schemas"
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

// TableIndices declares the unique constraints on (project_id, issue_id) and (project_board_id, sorting).
// Use naked index names so xorm prefixes them per-table (avoids SQLite's
// database-scoped index name collision during RecreateTable; see Column.TableIndices).
func (*ProjectIssue) TableIndices() []*schemas.Index {
	indices := make([]*schemas.Index, 0, 2)

	piUnique := schemas.NewIndex("project_issue", schemas.UniqueType)
	piUnique.AddColumn("project_id", "issue_id")
	indices = append(indices, piUnique)

	csUnique := schemas.NewIndex("column_sorting", schemas.UniqueType)
	csUnique.AddColumn("project_board_id", "sorting")
	indices = append(indices, csUnique)

	return indices
}

func init() {
	db.RegisterModel(new(ProjectIssue))
}

func deleteProjectIssuesByProjectID(ctx context.Context, projectID int64) error {
	_, err := db.GetEngine(ctx).Where("project_id=?", projectID).Delete(&ProjectIssue{})
	return err
}

// CountIssuesOnProject returns how many of the given issue ids are on the project
func CountIssuesOnProject(ctx context.Context, projectID int64, issueIDs []int64) (int64, error) {
	return db.GetEngine(ctx).
		Where("project_id=?", projectID).
		In("issue_id", issueIDs).
		Count(new(ProjectIssue))
}

// GetColumnIssueNextSorting returns the sorting value to append an issue at the end of the column.
func GetColumnIssueNextSorting(ctx context.Context, projectID, columnID int64) (int64, error) {
	res := struct {
		MaxSorting int64
		IssueCount int64
	}{}
	if _, err := db.GetEngine(ctx).Select("max(sorting) AS max_sorting, count(*) AS issue_count").
		Table("project_issue").
		Where("project_id=?", projectID).
		And("project_board_id=?", columnID).
		Get(&res); err != nil {
		return 0, err
	}
	return util.Iif(res.IssueCount > 0, res.MaxSorting+1, 0), nil
}

// DeleteAllProjectIssueByIssueIDsAndProjectIDs delete all project's issues by issue's and project's ids
func DeleteAllProjectIssueByIssueIDsAndProjectIDs(ctx context.Context, issueIDs, projectIDs []int64) error {
	_, err := db.GetEngine(ctx).In("project_id", projectIDs).In("issue_id", issueIDs).Delete(&ProjectIssue{})
	return err
}
