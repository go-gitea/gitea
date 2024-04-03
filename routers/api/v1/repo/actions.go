// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

// ListActionTasks list all the actions of a repository
func ListActionTasks(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/actions/tasks repository ListActionTasks
	// ---
	// summary: List a repository's action tasks
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results, default maximum page size is 50
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/TasksList"
	page := ctx.FormInt("page")
	limit := convert.ToCorrectPageSize(ctx.FormInt("limit"))

	opts := actions_model.FindTaskOptions{
		RepoID: ctx.Repo.Repository.ID,
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: limit,
		},
		Status:      actions_model.StatusUnknown, // Unknown means all
		IDOrderDesc: true,
	}
	tasks, err := db.Find[actions_model.ActionTask](ctx, opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ListActionTasks", err)
		return
	}

	res := new(api.ActionTaskResponse)

	res.Entries = make([]*api.ActionTask, len(tasks))
	for i := range tasks {
		res.Entries[i] = convert.ToActionTask(ctx, ctx.Repo.Repository, tasks[i])
	}

	opts = actions_model.FindTaskOptions{
		RepoID:      ctx.Repo.Repository.ID,
		Status:      actions_model.StatusUnknown, // Unknown means all
		IDOrderDesc: true,
	}
	tasks, res.TotalCount, err = db.FindAndCount[actions_model.ActionTask](ctx, opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ListActionTasks", err)
		return
	}

	ctx.JSON(http.StatusOK, &res)
}
