// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	perm "code.gitea.io/gitea/models/perm/access"
	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/convert"
)

func GetProjectBoard(ctx *context.APIContext) {
	// swagger:operation GET /projects/boards/{id} board boardGetProjectBoard
	// ---
	// summary: Get project board
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the board
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectBoard"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	board, err := project_model.GetBoard(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		if project_model.IsErrProjectBoardNotExist(err) {
			ctx.Error(http.StatusNotFound, "GetProjectBoard", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetProjectBoard", err)
		}
		return
	}

	board.LoadProject(ctx)
	board.Project.LoadRepo(ctx)
	permission, err := perm.GetUserRepoPermission(ctx, board.Project.Repo, ctx.Doer)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetProjectBoard", err)
		return
	}

	if !permission.CanRead(unit.TypeProjects) {
		ctx.Error(http.StatusUnauthorized, "GetProjectBoard", "board doesn't belong to repository")
		return
	}

	ctx.JSON(http.StatusOK, convert.ToAPIProjectBoard(board))
}

func UpdateProjectBoard(ctx *context.APIContext) {
	// swagger:operation PATCH /projects/boards/{id} board boardUpdateProjectBoard
	// ---
	// summary: Update project board
	// produces:
	// - application/json
	// consumes:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project board
	//   type: string
	//   required: true
	// - name: board
	//   in: body
	//   required: true
	//   schema: { "$ref": "#/definitions/UpdateProjectBoardPayload" }
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectBoard"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	form := web.GetForm(ctx).(*api.UpdateProjectBoardPayload)

	board, err := project_model.GetBoard(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		ctx.Error(http.StatusNotFound, "GetProjectBoard", err)
		return
	}

	board.LoadProject(ctx)
	board.Project.LoadRepo(ctx)
	permission, err := perm.GetUserRepoPermission(ctx, board.Project.Repo, ctx.Doer)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetProjectBoard", err)
		return
	}

	if !permission.CanWrite(unit.TypeProjects) {
		ctx.Error(http.StatusUnauthorized, "GetProjectBoard", "board doesn't belong to repository")
		return
	}

	board.Title = form.Title
	if board.Color != form.Color {
		board.Color = form.Color
	}

	if err = project_model.UpdateBoard(ctx, board); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateProjectBoard", err)
		return
	}

	board, err = project_model.GetBoard(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		ctx.Error(http.StatusNotFound, "GetProjectBoard", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToAPIProjectBoard(board))
}

func DeleteProjectBoard(ctx *context.APIContext) {
	// swagger:operation DELETE /projects/boards/{id} board boardDeleteProjectBoard
	// ---
	// summary: Delete project board
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project board
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "description": "Project board deleted"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	board, err := project_model.GetBoard(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		ctx.Error(http.StatusNotFound, "GetProjectBoard", err)
		return
	}

	board.LoadProject(ctx)
	board.Project.LoadRepo(ctx)
	permission, err := perm.GetUserRepoPermission(ctx, board.Project.Repo, ctx.Doer)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetProjectBoard", err)
		return
	}

	if !permission.CanWrite(unit.TypeProjects) {
		ctx.Error(http.StatusUnauthorized, "GetProjectBoard", "board doesn't belong to repository")
		return
	}

	if err := project_model.DeleteBoardByID(ctx, ctx.ParamsInt64(":id")); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteProjectBoard", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

func ListProjectBoards(ctx *context.APIContext) {
	// swagger:operation GET /projects/{id}/boards board boardGetProjectBoards
	// ---
	// summary: Get project boards
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: string
	//   required: true
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/ProjectBoardList"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	boards, count, err := project_model.GetBoardsAndCount(ctx, ctx.ParamsInt64(":id"))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Boards", err)
		return
	}

	ctx.SetLinkHeader(int(count), setting.UI.IssuePagingNum)
	ctx.SetTotalCountHeader(count)

	apiBoards, err := convert.ToAPIProjectBoardList(boards)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.JSON(http.StatusOK, apiBoards)
}

func CreateProjectBoard(ctx *context.APIContext) {
	// swagger:operation POST /projects/{id}/boards board boardCreateProjectBoard
	// ---
	// summary: Create project board
	// produces:
	// - application/json
	// consumes:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the project
	//   type: string
	//   required: true
	// - name: board
	//   in: body
	//   required: true
	//   schema: { "$ref": "#/definitions/NewProjectBoardPayload" }
	// responses:
	//   "201":
	//     "$ref": "#/responses/ProjectBoard"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	form := web.GetForm(ctx).(*api.NewProjectBoardPayload)

	board := &project_model.Board{
		Title:     form.Title,
		Default:   form.Default,
		Sorting:   form.Sorting,
		Color:     form.Color,
		ProjectID: ctx.ParamsInt64(":id"),
		CreatorID: ctx.Doer.ID,
	}

	var err error
	if err = project_model.NewBoard(board); err != nil {
		ctx.Error(http.StatusInternalServerError, "CreateBoard", err)
		return
	}

	board, err = project_model.GetBoard(ctx, board.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetBoard", err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToAPIProjectBoard(board))
}
