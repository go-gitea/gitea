// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"strings"

	issues_model "gitea.dev/models/issues"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	files_service "gitea.dev/services/repository/files"
)

func ApplyPullReviewSuggestions(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/pulls/{index}/suggestions/apply repository repoApplyPullReviewSuggestions
	// ---
	// summary: Apply pull request review suggestions in one commit
	// consumes:
	// - application/json
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
	// - name: index
	//   in: path
	//   description: index of the pull request
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/ApplyPullReviewSuggestionsOptions"
	// responses:
	//   "201":
	//     "$ref": "#/responses/ApplyPullReviewSuggestionsResponse"
	//   default:
	//     "$ref": "#/responses/error"

	pr, err := issues_model.GetPullRequestByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}

	form := web.GetForm[*api.ApplyPullReviewSuggestionsOptions](ctx)
	summary, description, _ := strings.Cut(util.NormalizeStringEOL(form.Message), "\n")
	commitID, err := files_service.ApplyCodeSuggestions(ctx, ctx.Doer, pr.ID, files_service.ApplyCodeSuggestionsOptions{
		CommentIDs:   form.CommentIDs,
		HeadCommitID: form.HeadCommitID,
		Summary:      summary,
		Description:  description,
	})
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	ctx.JSON(http.StatusCreated, api.ApplyPullReviewSuggestionsResponse{SHA: commitID})
}
