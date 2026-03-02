// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	repo_service "code.gitea.io/gitea/services/repository"
)

// Reparent promotes a fork to become a top-level repository
func Reparent(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/reparent repository repoReparent
	// ---
	// summary: Reparent a repo
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo to reparent
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to reparent
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   description: "Reparent Options"
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/ReparentRepoOption"
	// responses:
	//   "202":
	//     "$ref": "#/responses/Repository"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	opts := web.GetForm(ctx).(*api.ReparentRepoOption)

	targetOwner, err := user_model.GetUserByName(ctx, opts.NewOwner)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.APIError(http.StatusNotFound, "The target owner does not exist")
			return
		}
		ctx.APIErrorInternal(err)
		return
	}

	// Permission check:
	// 1. Initiator is instance admin
	// 2. Initiator is owner of source repo
	if !ctx.Doer.IsAdmin && !ctx.Repo.IsOwner() {
		ctx.APIError(http.StatusForbidden, "Only instance admins or the repository owner can initiate reparenting")
		return
	}

	repo, err := repo_service.StartRepositoryReparent(ctx, ctx.Doer, ctx.Repo.Repository, targetOwner.ID)
	if err != nil {
		switch {
		case repo_model.IsErrRepoTransferInProgress(err):
			ctx.APIError(http.StatusConflict, err)
		default:
			ctx.APIErrorInternal(err)
		}
		return
	}

	// Auto-accept ONLY if initiator is instance admin
	if ctx.Doer.IsAdmin {
		if err := repo_service.AcceptReparent(ctx, ctx.Doer, repo); err == nil {
			ctx.JSON(http.StatusOK, convert.ToRepo(ctx, repo, ctx.Repo.Permission))
			return
		}
		// If auto-accept fails, it stays pending (202)
	}

	ctx.JSON(http.StatusAccepted, convert.ToRepo(ctx, repo, ctx.Repo.Permission))
}

// AcceptReparent accept a reparenting request
func AcceptReparent(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/reparent/accept repository acceptRepoReparent
	// ---
	// summary: Accept a reparenting request
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo to accept reparenting for
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to accept reparenting for
	//   type: string
	//   required: true
	// responses:
	//   "202":
	//     "$ref": "#/responses/Repository"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	repoTransfer, err := repo_model.GetPendingRepositoryTransfer(ctx, ctx.Repo.Repository)
	if err != nil {
		if repo_model.IsErrNoPendingTransfer(err) || errors.Is(err, repo_model.ErrNoPendingRepoTransfer{}) {
			ctx.APIError(http.StatusNotFound, err)
			return
		}
		ctx.APIErrorInternal(err)
		return
	}

	if !repoTransfer.CanUserAcceptOrRejectTransfer(ctx, ctx.Doer) {
		ctx.APIError(http.StatusForbidden, "Only the repository owner can accept reparenting")
		return
	}

	err = repo_service.AcceptReparent(ctx, ctx.Doer, ctx.Repo.Repository)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusAccepted, convert.ToRepo(ctx, ctx.Repo.Repository, ctx.Repo.Permission))
}

// RejectReparent reject a reparenting request
func RejectReparent(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/reparent/reject repository rejectRepoReparent
	// ---
	// summary: Reject a reparenting request
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo to reject reparenting for
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to reject reparenting for
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Repository"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	repoTransfer, err := repo_model.GetPendingRepositoryTransfer(ctx, ctx.Repo.Repository)
	if err != nil {
		if repo_model.IsErrNoPendingTransfer(err) || errors.Is(err, repo_model.ErrNoPendingRepoTransfer{}) {
			ctx.APIError(http.StatusNotFound, err)
			return
		}
		ctx.APIErrorInternal(err)
		return
	}

	if !repoTransfer.CanUserAcceptOrRejectTransfer(ctx, ctx.Doer) && repoTransfer.DoerID != ctx.Doer.ID {
		ctx.APIError(http.StatusForbidden, "Only the repository owner or the initiator can reject/cancel reparenting")
		return
	}

	err = repo_service.CancelRepositoryReparent(ctx, ctx.Doer, ctx.Repo.Repository)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToRepo(ctx, ctx.Repo.Repository, ctx.Repo.Permission))
}
