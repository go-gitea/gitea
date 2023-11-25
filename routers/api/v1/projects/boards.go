// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package projects

import (
	"net/http"

	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/convert"
)

func ListProjectBoards(ctx *context.APIContext) {
	// swagger:operation GET /projects/{projectId}/boards board boardGetProjectBoards
	// ---
	// summary: Get project boards
	// produces:
	// - application/json
	// parameters:
	//   - name: id
	//     in: path
	//     description: id of the project
	//     type: string
	//     required: true
	//   - name: page
	//     in: query
	//     description: page number of results to return (1-based)
	//     type: integer
	//   - name: limit
	//     in: query
	//     description: page size of results
	//     type: integer
	//
	// responses:
	//
	//	"200":
	//	  "$ref": "#/responses/ProjectBoardList"
	//	"403":
	//	  "$ref": "#/responses/forbidden"
	//	"404":
	//	  "$ref": "#/responses/notFound"
	project_id := ctx.ParamsInt64(":projectId")
	project, err := project_model.GetProjectByID(ctx, project_id)
	if err != nil {
		ctx.Error(http.StatusNotFound, "ListProjectBoards", err)
		return
	}

	boards, count, err := project.GetBoardsAndCount(ctx)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.SetLinkHeader(int(count), setting.UI.IssuePagingNum)
	ctx.SetTotalCountHeader(count)

	apiBoards, err := convert.ToApiProjectBoardList(ctx, boards)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.JSON(200, apiBoards)

}

func CreateProjectBoard(ctx *context.APIContext) {
	// swagger:operation POST /projects/{projectId}/boards board boardCreateProjectBoard
	// ---
	// summary: Create project board
	// produces:
	// - application/json
	// consumes:
	// - application/json
	// parameters:
	//   - name: id
	//     in: path
	//     description: id of the project
	//     type: string
	//     required: true
	//   - name: board
	//     in: body
	//     required: true
	//     schema: { "$ref": "#/definitions/NewProjectBoardPayload" }
	//
	// responses:
	//
	//	"201":
	//	  "$ref": "#/responses/ProjectBoard"
	//	"403":
	//	  "$ref": "#/responses/forbidden"
	//	"404":
	//	  "$ref": "#/responses/notFound"
	form := web.GetForm(ctx).(*api.NewProjectBoardPayload)

	board := &project_model.Board{
		Title:     form.Title,
		Default:   form.Default,
		Sorting:   form.Sorting,
		Color:     form.Color,
		ProjectID: ctx.ParamsInt64(":projectId"),
		CreatorID: ctx.Doer.ID,
	}

	if err := project_model.NewBoard(ctx, board); err != nil {
		ctx.InternalServerError(err)
		return
	}

	board, err := project_model.GetBoard(ctx, board.ID)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToAPIProjectBoard(ctx, board))
}

func GetProjectBoard(ctx *context.APIContext) {
	// swagger:operation GET /projects/boards/{boardId} board boardGetProjectBoard
	// ---
	// summary: Get project board
	// produces:
	// - application/json
	// parameters:
	//   - name: id
	//     in: path
	//     description: id of the board
	//     type: string
	//     required: true
	//
	// responses:
	//
	//	"200":
	//	  "$ref": "#/responses/ProjectBoard"
	//	"403":
	//	  "$ref": "#/responses/forbidden"
	//	"404":
	//	  "$ref": "#/responses/notFound"
	board, err := project_model.GetBoard(ctx, ctx.ParamsInt64(":boardId"))
	if err != nil {
		if project_model.IsErrProjectBoardNotExist(err) {
			ctx.Error(http.StatusNotFound, "GetProjectBoard", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetProjectBoard", err)
		}
		return
	}

	ctx.JSON(http.StatusOK, convert.ToAPIProjectBoard(ctx, board))
}

func UpdateProjectBoard(ctx *context.APIContext) {
	// swagger:operation PATCH /projects/boards/{boardId} board boardUpdateProjectBoard
	// ---
	// summary: Update project board
	// produces:
	// - application/json
	// consumes:
	// - application/json
	// parameters:
	//   - name: id
	//     in: path
	//     description: id of the project board
	//     type: string
	//     required: true
	//   - name: board
	//     in: body
	//     required: true
	//     schema: { "$ref": "#/definitions/UpdateProjectBoardPayload" }
	//
	// responses:
	//
	//	"200":
	//	  "$ref": "#/responses/ProjectBoard"
	//	"403":
	//	  "$ref": "#/responses/forbidden"
	//	"404":
	//	  "$ref": "#/responses/notFound"

	form := web.GetForm(ctx).(*api.UpdateProjectBoardPayload)

	board, err := project_model.GetBoard(ctx, ctx.ParamsInt64(":boardId"))
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	if board.Title != form.Title {
		board.Title = form.Title
	}
	if board.Color != form.Color {
		board.Color = form.Color
	}

	if err := project_model.UpdateBoard(ctx, board); err != nil {
		ctx.InternalServerError(err)
		return
	}

	board, err = project_model.GetBoard(ctx, board.ID)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.JSON(200, convert.ToAPIProjectBoard(ctx, board))
}

func DeleteProjectBoard(ctx *context.APIContext) {
	// swagger:operation DELETE /projects/boards/{boardId} board boardDeleteProjectBoard
	// ---
	// summary: Delete project board
	// produces:
	// - application/json
	// parameters:
	//   - name: id
	//     in: path
	//     description: id of the project board
	//     type: string
	//     required: true
	//
	// responses:
	//
	//	"204":
	//	  "description": "Project board deleted"
	//	"403":
	//	  "$ref": "#/responses/forbidden"
	//	"404":
	//	  "$ref": "#/responses/notFound"
	if err := project_model.DeleteBoardByID(ctx, ctx.ParamsInt64(":boardId")); err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.Status(http.StatusNoContent)
}
