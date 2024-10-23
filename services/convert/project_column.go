// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	column_model "code.gitea.io/gitea/models/project"
	api "code.gitea.io/gitea/modules/structs"
)

// ToProject converts a models.Project to api.Project
func ToColumn(ctx context.Context, column *column_model.Column) *api.Column {
	if column == nil {
		return nil
	}

	return &api.Column{
		ID:    column.ID,
		Title: column.Title,
		Color: column.Color,
	}
}

func ToColumns(ctx context.Context, columns column_model.ColumnList) []*api.Column {
	if columns == nil {
		return nil
	}

	var apiColumns []*api.Column
	for _, column := range columns {
		apiColumns = append(apiColumns, ToColumn(ctx, column))
	}
	return apiColumns
}
