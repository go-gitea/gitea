// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/services/context"
	project_service "code.gitea.io/gitea/services/projects"
)

// MoveColumns moves or keeps columns in a project and sorts them inside that project
func MoveColumns(ctx *context.Context) {
	project, err := project_model.GetProjectByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
		return
	}
	if !project.CanBeAccessedByOwnerRepo(ctx.ContextUser.ID, ctx.Repo.Repository) {
		ctx.NotFound(nil)
		return
	}

	type movedColumnsForm struct {
		Columns []struct {
			ColumnID int64 `json:"columnID"`
			Sorting  int64 `json:"sorting"`
		} `json:"columns"`
	}

	form := &movedColumnsForm{}
	if err = json.NewDecoder(ctx.Req.Body).Decode(&form); err != nil {
		ctx.ServerError("DecodeMovedColumnsForm", err)
		return
	}

	sortedColumnIDs := make(map[int64]int64)
	for _, column := range form.Columns {
		sortedColumnIDs[column.ColumnID] = column.Sorting
	}

	if err = project_service.ReorderColumns(ctx, project, sortedColumnIDs); err != nil {
		ctx.ServerError("ReorderColumns", err)
		return
	}

	ctx.JSONOK()
}
