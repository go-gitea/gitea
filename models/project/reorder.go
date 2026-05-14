// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
)

// applyReorder rewrites `sorting` in two passes so the unique index that
// includes sorting is never transiently violated: pass 1 parks every target
// row at a distinct negative value, pass 2 writes the final sorting.
func applyReorder(
	ctx context.Context,
	table, whereColumn string,
	sortings map[int64]int64,
	extraSetSQL string, extraSetVal any,
) error {
	sess := db.GetEngine(ctx)

	parkQuery := fmt.Sprintf("UPDATE `%s` SET sorting=? WHERE `%s`=?", table, whereColumn)
	parked := int64(-1)
	for id := range sortings {
		if _, err := sess.Exec(parkQuery, parked, id); err != nil {
			return err
		}
		parked--
	}

	var finalQuery string
	if extraSetSQL == "" {
		finalQuery = fmt.Sprintf("UPDATE `%s` SET sorting=? WHERE `%s`=?", table, whereColumn)
		for id, sorting := range sortings {
			if _, err := sess.Exec(finalQuery, sorting, id); err != nil {
				return err
			}
		}
		return nil
	}

	finalQuery = fmt.Sprintf("UPDATE `%s` SET %s, sorting=? WHERE `%s`=?", table, extraSetSQL, whereColumn)
	for id, sorting := range sortings {
		if _, err := sess.Exec(finalQuery, extraSetVal, sorting, id); err != nil {
			return err
		}
	}
	return nil
}

// SetColumnSortings rewrites sortings for the given columns in a project
func SetColumnSortings(ctx context.Context, projectID int64, sortings map[int64]int64) error {
	if len(sortings) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(sortings))
	for id := range sortings {
		ids = append(ids, id)
	}
	return db.WithTx(ctx, func(ctx context.Context) error {
		got, err := GetColumnsByIDs(ctx, projectID, ids)
		if err != nil {
			return err
		}
		if len(got) != len(sortings) {
			return fmt.Errorf("SetColumnSortings: %d of %d columns do not belong to project %d",
				len(sortings)-len(got), len(sortings), projectID)
		}
		return applyReorder(ctx, "project_board", "id", sortings, "", nil)
	})
}

// SetIssueSortingsInColumn rewrites sortings for the given issues and moves each into columnID
func SetIssueSortingsInColumn(ctx context.Context, columnID int64, sortings map[int64]int64) error {
	if len(sortings) == 0 {
		return nil
	}
	return db.WithTx(ctx, func(ctx context.Context) error {
		return applyReorder(ctx, "project_issue", "issue_id", sortings, "project_board_id=?", columnID)
	})
}

// MaxIssueSortingInColumn returns the largest sorting in the column and whether any row exists
func MaxIssueSortingInColumn(ctx context.Context, columnID int64) (maxSorting int64, hasAny bool, err error) {
	var res struct {
		MaxSorting int64
		IssueCount int64
	}
	_, err = db.GetEngine(ctx).Select(
		"COALESCE(MAX(sorting), -1) AS max_sorting, COUNT(*) AS issue_count",
	).Table("project_issue").Where("project_board_id=?", columnID).Get(&res)
	return res.MaxSorting, res.IssueCount > 0, err
}

// AppendIssueToColumn puts the issue at sorting = max+1 within the column and returns that sorting
func AppendIssueToColumn(ctx context.Context, projectID, columnID, issueID int64) (int64, error) {
	var sorting int64
	err := db.WithTx(ctx, func(ctx context.Context) error {
		sess := db.GetEngine(ctx)

		var res struct {
			MaxSorting int64
			IssueCount int64
		}
		if _, err := sess.Select("COALESCE(MAX(sorting), -1) AS max_sorting, COUNT(*) AS issue_count").
			Table("project_issue").
			Where("project_board_id=?", columnID).
			Get(&res); err != nil {
			return err
		}
		if res.IssueCount > 0 {
			sorting = res.MaxSorting + 1
		} else {
			sorting = 0
		}

		has, err := sess.Where("project_id=? AND issue_id=?", projectID, issueID).
			Exist(new(ProjectIssue))
		if err != nil {
			return err
		}
		if has {
			_, err := sess.Exec(
				"UPDATE `project_issue` SET project_board_id=?, sorting=? WHERE project_id=? AND issue_id=?",
				columnID, sorting, projectID, issueID,
			)
			return err
		}

		_, err = sess.Insert(&ProjectIssue{
			IssueID:         issueID,
			ProjectID:       projectID,
			ProjectColumnID: columnID,
			Sorting:         sorting,
		})
		return err
	})
	return sorting, err
}
