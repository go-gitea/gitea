// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/repository/files"
)

// ApplyDiffPatch handles API call for applying a patch
func ApplyDiffPatch(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/diffpatch repository repoApplyDiffPatch
	// ---
	// summary: Apply diff patch to repository
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
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/UpdateFileOptions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/FileResponse"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"
	apiOpts, changeRepoFileOpts := getAPIChangeRepoFileOptions[*api.ApplyDiffPatchFileOptions](ctx)
	opts := &files.ApplyDiffPatchOptions{
		Content: apiOpts.Content,
		Message: util.IfZero(apiOpts.Message, "apply-patch"),

		OldBranch: changeRepoFileOpts.OldBranch,
		NewBranch: changeRepoFileOpts.NewBranch,
		Committer: changeRepoFileOpts.Committer,
		Author:    changeRepoFileOpts.Author,
		Dates:     changeRepoFileOpts.Dates,
		Signoff:   changeRepoFileOpts.Signoff,
	}

	fileResponse, err := files.ApplyDiffPatch(ctx, ctx.Repo.Repository, ctx.Doer, opts)
	if err != nil {
		handleChangeRepoFilesError(ctx, err)
	} else {
		ctx.JSON(http.StatusCreated, fileResponse)
	}
}
