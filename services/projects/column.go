// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"errors"

	"code.gitea.io/gitea/models/db"
	project_model "code.gitea.io/gitea/models/project"
)

// ReorderColumns rewrites the sortings of the given project's columns
func ReorderColumns(ctx context.Context, project *project_model.Project, sortings map[int64]int64) error {
	return project_model.SetColumnSortings(ctx, project.ID, sortings)
}

// DeleteColumn removes a non-default column and moves its issues to the default column
func DeleteColumn(ctx context.Context, column *project_model.Column) error {
	if column.Default {
		return errors.New("DeleteColumn: cannot delete default column")
	}
	return db.WithTx(ctx, func(ctx context.Context) error {
		project, err := project_model.GetProjectByID(ctx, column.ProjectID)
		if err != nil {
			return err
		}
		defaultColumn, err := project.MustDefaultColumn(ctx)
		if err != nil {
			return err
		}

		issues, err := column.GetIssues(ctx)
		if err != nil {
			return err
		}

		if len(issues) > 0 {
			maxSorting, hasAny, err := project_model.MaxIssueSortingInColumn(ctx, defaultColumn.ID)
			if err != nil {
				return err
			}
			start := int64(0)
			if hasAny {
				start = maxSorting + 1
			}

			sortings := make(map[int64]int64, len(issues))
			for i, pi := range issues {
				sortings[pi.IssueID] = start + int64(i)
			}
			if err := project_model.SetIssueSortingsInColumn(ctx, defaultColumn.ID, sortings); err != nil {
				return err
			}
		}

		return project_model.DeleteColumnRow(ctx, column)
	})
}
