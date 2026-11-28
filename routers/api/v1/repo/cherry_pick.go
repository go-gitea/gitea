// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"regexp"

	"gitea.dev/modules/git"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
	"gitea.dev/services/repository/files"
)

// CherryPick handles API call for cherry-picking a commit
func CherryPick(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/commits/{sha}/cherry-pick repository repoCherryPick
	// ---
	// summary: Cherry-pick a commit onto a branch
	// description: The commit is applied with a three-way merge. If it conflicts or the result would not change the branch, nothing is committed and status 409 is returned. With dry_run the merge is checked but nothing is pushed, so branch protection rules and hooks are not evaluated.
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
	// - name: sha
	//   in: path
	//   description: SHA of the commit to cherry-pick
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/CherryPickOptions"
	// responses:
	//   "201":
	//     "$ref": "#/responses/FileResponse"
	//   "204":
	//     "$ref": "#/responses/empty"
	//   default:
	//     "$ref": "#/responses/error"
	cherryPickOrRevert(ctx, false)
}

// RevertCommit handles API call for reverting a commit
func RevertCommit(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/commits/{sha}/revert repository repoRevertCommit
	// ---
	// summary: Revert a commit on a branch
	// description: The revert is applied with a three-way merge. If it conflicts or the result would not change the branch, nothing is committed and status 409 is returned. With dry_run the merge is checked but nothing is pushed, so branch protection rules and hooks are not evaluated.
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
	// - name: sha
	//   in: path
	//   description: SHA of the commit to revert
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/CherryPickOptions"
	// responses:
	//   "201":
	//     "$ref": "#/responses/FileResponse"
	//   "204":
	//     "$ref": "#/responses/empty"
	//   default:
	//     "$ref": "#/responses/error"
	cherryPickOrRevert(ctx, true)
}

var cherryPickSHAPattern = regexp.MustCompile(`^[0-9a-fA-F]{4,64}$`)

func cherryPickOrRevert(ctx *context.APIContext, revert bool) {
	apiOpts, changeRepoFileOpts := getAPIChangeRepoFileOptions[*structs.CherryPickOptions](ctx)
	sha := ctx.PathParam("sha")
	if !cherryPickSHAPattern.MatchString(sha) {
		ctx.APIError(http.StatusUnprocessableEntity, "sha must be a hexadecimal commit ID")
		return
	}
	commit, err := ctx.Repo.GitRepo.GetCommit(ctx, sha)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	if commit.ParentCount() > 1 {
		ctx.APIError(http.StatusUnprocessableEntity, "merge commits can't be cherry-picked or reverted")
		return
	}
	sha = commit.ID.String()
	opts := &files.ApplyDiffPatchOptions{
		Content: sha,
		Message: changeRepoFileOpts.Message,

		OldBranch: changeRepoFileOpts.OldBranch,
		NewBranch: changeRepoFileOpts.NewBranch,
		Committer: changeRepoFileOpts.Committer,
		Author:    changeRepoFileOpts.Author,
		Dates:     changeRepoFileOpts.Dates,
		Signoff:   changeRepoFileOpts.Signoff,
		DryRun:    apiOpts.DryRun,

		FailOnEmpty: true,
	}
	if opts.Message == "" {
		opts.Message = util.Iif(revert, "Revert: ", "Cherry-pick: ") + sha
	}

	fileResponse, err := files.CherryPick(ctx, ctx.Repo.Repository, ctx.Doer, revert, opts)
	if err != nil {
		handleChangeRepoFilesError(ctx, err)
		return
	}
	if apiOpts.DryRun {
		ctx.Status(http.StatusNoContent)
		return
	}
	ctx.JSON(http.StatusCreated, fileResponse)
}
