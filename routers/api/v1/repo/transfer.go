// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	repo_service "code.gitea.io/gitea/services/repository"
)

// Transfer transfers the ownership of a repository
func Transfer(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/transfer repository repoTransfer
	// ---
	// summary: Transfer a repo ownership
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo to transfer
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to transfer
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   description: "Transfer Options"
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/TransferRepoOption"
	// responses:
	//   "202":
	//     "$ref": "#/responses/Repository"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	opts := web.GetForm(ctx).(*api.TransferRepoOption)

	newOwner, err := user_model.GetUserByName(ctx, opts.NewOwner)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.APIError(http.StatusNotFound, "The new owner does not exist or cannot be found")
			return
		}
		ctx.APIErrorInternal(err)
		return
	}

	if newOwner.Type == user_model.UserTypeOrganization {
		if !ctx.Doer.IsAdmin && newOwner.Visibility == api.VisibleTypePrivate && !organization.OrgFromUser(newOwner).HasMemberWithUserID(ctx, ctx.Doer.ID) {
			// The user shouldn't know about this organization
			ctx.APIError(http.StatusNotFound, "The new owner does not exist or cannot be found")
			return
		}
	}

	var teams []*organization.Team
	if opts.TeamIDs != nil {
		if !newOwner.IsOrganization() {
			ctx.APIError(http.StatusUnprocessableEntity, "Teams can only be added to organization-owned repositories")
			return
		}

		org := convert.ToOrganization(ctx, organization.OrgFromUser(newOwner))
		for _, tID := range *opts.TeamIDs {
			team, err := organization.GetTeamByID(ctx, tID)
			if err != nil {
				ctx.APIError(http.StatusUnprocessableEntity, fmt.Errorf("team %d not found", tID))
				return
			}

			if team.OrgID != org.ID {
				ctx.APIError(http.StatusForbidden, fmt.Errorf("team %d belongs not to org %d", tID, org.ID))
				return
			}

			teams = append(teams, team)
		}
	}

	if ctx.Repo.GitRepo != nil {
		_ = ctx.Repo.GitRepo.Close()
		ctx.Repo.GitRepo = nil
	}

	oldFullname := ctx.Repo.Repository.FullName()

	if err := repo_service.StartRepositoryTransfer(ctx, ctx.Doer, newOwner, ctx.Repo.Repository, teams); err != nil {
		if repo_model.IsErrRepoTransferInProgress(err) {
			ctx.APIError(http.StatusConflict, err)
			return
		}

		if repo_model.IsErrRepoAlreadyExist(err) {
			ctx.APIError(http.StatusUnprocessableEntity, err)
			return
		}

		if errors.Is(err, user_model.ErrBlockedUser) {
			ctx.APIError(http.StatusForbidden, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if ctx.Repo.Repository.Status == repo_model.RepositoryPendingTransfer {
		log.Trace("Repository transfer initiated: %s -> %s", oldFullname, ctx.Repo.Repository.FullName())
		ctx.JSON(http.StatusCreated, convert.ToRepo(ctx, ctx.Repo.Repository, access_model.Permission{AccessMode: perm.AccessModeAdmin}))
		return
	}

	log.Trace("Repository transferred: %s -> %s", oldFullname, ctx.Repo.Repository.FullName())
	ctx.JSON(http.StatusAccepted, convert.ToRepo(ctx, ctx.Repo.Repository, access_model.Permission{AccessMode: perm.AccessModeAdmin}))
}

// AcceptTransfer accept a repo transfer
func AcceptTransfer(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/transfer/accept repository acceptRepoTransfer
	// ---
	// summary: Accept a repo transfer
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo to transfer
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to transfer
	//   type: string
	//   required: true
	// responses:
	//   "202":
	//     "$ref": "#/responses/Repository"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	err := repo_service.AcceptTransferOwnership(ctx, ctx.Repo.Repository, ctx.Doer)
	if err != nil {
		switch {
		case repo_model.IsErrNoPendingTransfer(err):
			ctx.APIError(http.StatusNotFound, err)
		case errors.Is(err, util.ErrPermissionDenied):
			ctx.APIError(http.StatusForbidden, err)
		default:
			ctx.APIErrorInternal(err)
		}
		return
	}

	ctx.JSON(http.StatusAccepted, convert.ToRepo(ctx, ctx.Repo.Repository, ctx.Repo.Permission))
}

// RejectTransfer reject a repo transfer
func RejectTransfer(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/transfer/reject repository rejectRepoTransfer
	// ---
	// summary: Reject a repo transfer
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo to transfer
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to transfer
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Repository"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	err := repo_service.RejectRepositoryTransfer(ctx, ctx.Repo.Repository, ctx.Doer)
	if err != nil {
		switch {
		case repo_model.IsErrNoPendingTransfer(err):
			ctx.APIError(http.StatusNotFound, err)
		case errors.Is(err, util.ErrPermissionDenied):
			ctx.APIError(http.StatusForbidden, err)
		default:
			ctx.APIErrorInternal(err)
		}
		return
	}

	ctx.JSON(http.StatusOK, convert.ToRepo(ctx, ctx.Repo.Repository, ctx.Repo.Permission))
}
