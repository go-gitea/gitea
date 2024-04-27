// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"errors"

	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/services/context"
)

// MoveColumns moves or keeps columns in a project and sorts them inside that project
func MoveColumns(ctx *context.Context) {
	project, err := project_model.GetProjectByID(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetProjectByID", project_model.IsErrProjectNotExist, err)
		return
	}
	if project.OwnerID > 0 && project.OwnerID != ctx.ContextUser.ID {
		ctx.NotFound("InvalidOwnerID", nil)
		return
	}
	if project.RepoID > 0 && project.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound("InvalidRepoID", nil)
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

	columnIDs := make([]int64, 0, len(form.Columns))
	sortedColumnIDs := make(map[int64]int64)
	for _, column := range form.Columns {
		columnIDs = append(columnIDs, column.ColumnID)
		sortedColumnIDs[column.Sorting] = column.ColumnID
	}
	movedColumns, err := project_model.GetColumnsByIDs(ctx, columnIDs)
	if err != nil {
		ctx.NotFoundOrServerError("GetColumnsByIDs", issues_model.IsErrIssueNotExist, err)
		return
	}

	if len(movedColumns) != len(form.Columns) {
		ctx.ServerError("some columns do not exist", errors.New("some columns do not exist"))
		return
	}

	for _, column := range movedColumns {
		if column.ProjectID != project.ID {
			ctx.ServerError("Some column's projectID is not equal to project's ID", errors.New("Some column's projectID is not equal to project's ID"))
			return
		}
	}

	if err = project_model.MoveColumnsOnProject(ctx, project, sortedColumnIDs); err != nil {
		ctx.ServerError("MoveColumnsOnProject", err)
		return
	}

	ctx.JSONOK()
}
