// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"

	"gitea.dev/models/db"
)

// CountColumns returns the total number of columns for a project
func CountColumns(ctx context.Context, projectID int64) (int64, error) {
	return db.GetEngine(ctx).Where("project_id=?", projectID).Count(&Column{})
}

// GetColumns returns a list of columns for a project with pagination
func GetColumns(ctx context.Context, projectID int64, opts db.ListOptions) (ColumnList, error) {
	columns := make([]*Column, 0, opts.PageSize)
	s := db.GetEngine(ctx).Where("project_id=?", projectID).OrderBy("sorting, id")
	if !opts.IsListAll() {
		db.SetSessionPagination(s, &opts)
	}
	if err := s.Find(&columns); err != nil {
		return nil, err
	}
	return columns, nil
}

func GetColumnsByIDs(ctx context.Context, projectID int64, columnsIDs []int64) (ColumnList, error) {
	columns := make([]*Column, 0, 5)
	if len(columnsIDs) == 0 {
		return columns, nil
	}
	if err := db.GetEngine(ctx).
		Where("project_id =?", projectID).
		In("id", columnsIDs).
		OrderBy("sorting").Find(&columns); err != nil {
		return nil, err
	}
	return columns, nil
}
