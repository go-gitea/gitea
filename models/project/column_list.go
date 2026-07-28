// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"context"

	"gitea.dev/models/db"

	"xorm.io/builder"
)

// SearchColumnOptions selects a project's columns in board order.
type SearchColumnOptions struct {
	db.ListOptions
	ProjectID int64
	ColumnIDs []int64
}

func (opts SearchColumnOptions) ToConds() builder.Cond {
	cond := builder.NewCond().And(builder.Eq{"project_id": opts.ProjectID})
	if len(opts.ColumnIDs) > 0 {
		cond = cond.And(builder.In("id", opts.ColumnIDs))
	}
	return cond
}

func (opts SearchColumnOptions) ToOrders() string {
	return "sorting, id"
}

// GetProjectColumns returns a project's columns, paginated when the options ask for it.
func GetProjectColumns(ctx context.Context, projectID int64, opts db.ListOptions) (ColumnList, error) {
	return db.Find[Column](ctx, SearchColumnOptions{ListOptions: opts, ProjectID: projectID})
}

// GetColumnsByIDs returns the named columns of a project, in board order.
func GetColumnsByIDs(ctx context.Context, projectID int64, columnIDs []int64) (ColumnList, error) {
	if len(columnIDs) == 0 {
		return ColumnList{}, nil
	}
	return db.Find[Column](ctx, SearchColumnOptions{
		ListOptions: db.ListOptionsAll,
		ProjectID:   projectID,
		ColumnIDs:   columnIDs,
	})
}
