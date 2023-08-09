// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"time"

	"code.gitea.io/gitea/models"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
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
	apiOpts := web.GetForm(ctx).(*api.ApplyDiffPatchFileOptions)

	opts := &files.ApplyDiffPatchOptions{
		Content:   apiOpts.Content,
		SHA:       apiOpts.SHA,
		Message:   apiOpts.Message,
		OldBranch: apiOpts.BranchName,
		NewBranch: apiOpts.NewBranchName,
		Committer: &files.IdentityOptions{
			Name:  apiOpts.Committer.Name,
			Email: apiOpts.Committer.Email,
		},
		Author: &files.IdentityOptions{
			Name:  apiOpts.Author.Name,
			Email: apiOpts.Author.Email,
		},
		Dates: &files.CommitDateOptions{
			Author:    apiOpts.Dates.Author,
			Committer: apiOpts.Dates.Committer,
		},
		Signoff: apiOpts.Signoff,
	}
	if opts.Dates.Author.IsZero() {
		opts.Dates.Author = time.Now()
	}
	if opts.Dates.Committer.IsZero() {
		opts.Dates.Committer = time.Now()
	}

	if opts.Message == "" {
		opts.Message = "apply-patch"
	}

	if !canWriteFiles(ctx, apiOpts.BranchName) {
		ctx.Error(http.StatusInternalServerError, "ApplyPatch", repo_model.ErrUserDoesNotHaveAccessToRepo{
			UserID:   ctx.Doer.ID,
			RepoName: ctx.Repo.Repository.LowerName,
		})
		return
	}

	fileResponse, err := files.ApplyDiffPatch(ctx, ctx.Repo.Repository, ctx.Doer, opts)
	if err != nil {
		if models.IsErrUserCannotCommit(err) || models.IsErrFilePathProtected(err) {
			ctx.Error(http.StatusForbidden, "Access", err)
			return
		}
		if git_model.IsErrBranchAlreadyExists(err) || models.IsErrFilenameInvalid(err) || models.IsErrSHADoesNotMatch(err) ||
			models.IsErrFilePathInvalid(err) || models.IsErrRepoFileAlreadyExists(err) {
			ctx.Error(http.StatusUnprocessableEntity, "Invalid", err)
			return
		}
		if git_model.IsErrBranchNotExist(err) || git.IsErrBranchNotExist(err) {
			ctx.Error(http.StatusNotFound, "BranchDoesNotExist", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "ApplyPatch", err)
	} else {
		ctx.JSON(http.StatusCreated, fileResponse)
	}
}
