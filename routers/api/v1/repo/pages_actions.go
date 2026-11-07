// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"
	"net/url"

	actions_model "gitea.dev/models/actions"
	user_model "gitea.dev/models/user"
	pages_content "gitea.dev/modules/pages"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	pages_service "gitea.dev/services/pages"
)

func getPagesActionsTask(ctx *context.APIContext) (*actions_model.ActionTask, bool) {
	taskID, ok := user_model.GetActionsUserTaskID(ctx.Doer)
	if !ok || taskID <= 0 {
		ctx.APIError(http.StatusUnauthorized, "Pages deployment requires an Actions task token")
		return nil, false
	}
	task, err := actions_model.GetTaskByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.APIError(http.StatusUnauthorized, "Pages deployment task does not exist")
		} else {
			ctx.APIErrorInternal(err)
		}
		return nil, false
	}
	if task.RepoID != ctx.Repo.Repository.ID {
		ctx.APIError(http.StatusUnauthorized, "Pages deployment task does not belong to this repository")
		return nil, false
	}
	return task, true
}

// CreatePagesDeployment accepts the small Actions deploy-pages compatibility
// request and only returns after the artifact is validated and current.
func CreatePagesDeployment(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/pages/deployments repository repoCreatePagesDeployment
	// ---
	// summary: Create a Pages deployment from an Actions artifact
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repository
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/PagesDeploymentOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/PagesDeploymentResponse"
	//   default:
	//     "$ref": "#/responses/error"
	task, ok := getPagesActionsTask(ctx)
	if !ok {
		return
	}
	opt := web.GetForm[*api.PagesDeploymentOption](ctx)
	result, err := pages_service.CommitActionsArtifact(ctx, ctx.Repo.Repository.ID, task, pages_service.ActionsDeploymentRequest{
		ArtifactID: opt.ArtifactID, PagesBuildVersion: opt.PagesBuildVersion, OIDCToken: opt.OIDCToken, Preview: opt.Preview,
	})
	if err != nil {
		switch {
		case errors.Is(err, pages_service.ErrActionsDeploymentAuth):
			ctx.APIError(http.StatusForbidden, "Pages deployment is not authorized")
		case errors.Is(err, pages_service.ErrActionsPreview), errors.Is(err, pages_service.ErrActionsDeploymentRequest), errors.Is(err, pages_service.ErrActionsArtifact):
			ctx.APIError(http.StatusBadRequest, err.Error())
		case errors.Is(err, pages_service.ErrSourceChanged):
			ctx.APIError(http.StatusConflict, "Pages source changed while deploying")
		case errors.Is(err, pages_content.ErrArchiveTooLarge):
			ctx.APIError(http.StatusRequestEntityTooLarge, "Pages artifact exceeds a size limit")
		case errors.Is(err, pages_content.ErrInvalidArchive), errors.Is(err, pages_content.ErrArchivePath),
			errors.Is(err, pages_content.ErrArchiveEntry), errors.Is(err, pages_content.ErrArchiveSymlink):
			ctx.APIError(http.StatusBadRequest, "Invalid Pages artifact")
		default:
			ctx.APIErrorInternal(err)
		}
		return
	}
	owner := url.PathEscape(ctx.Repo.Repository.OwnerName)
	repoName := url.PathEscape(ctx.Repo.Repository.LowerName)
	statusURL := setting.AppURL + "api/v1/repos/" + owner + "/" + repoName + "/pages/deployments/" + result.Digest
	ctx.JSON(http.StatusOK, api.PagesDeploymentResponse{ID: result.Digest, StatusURL: statusURL, PageURL: result.PageURL})
}

// GetPagesDeployment reports the actual default-address publication status for
// the requested digest across all registered Pages instances.
func GetPagesDeployment(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/pages/deployments/{digest} repository repoGetPagesDeployment
	// ---
	// summary: Get the publication status of a Pages deployment
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repository
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: digest
	//   in: path
	//   description: SHA-256 digest returned when the deployment was created
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/PagesDeploymentStatus"
	//   default:
	//     "$ref": "#/responses/error"
	if _, ok := getPagesActionsTask(ctx); !ok {
		return
	}
	digest := ctx.PathParam("digest")
	status, err := pages_service.GetActionsDeploymentStatus(ctx, ctx.Repo.Repository.ID, digest)
	if errors.Is(err, util.ErrNotExist) {
		ctx.APIErrorNotFound()
		return
	}
	if errors.Is(err, pages_service.ErrActionsDeploymentRequest) {
		ctx.APIError(http.StatusBadRequest, "invalid deployment digest")
		return
	}
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, api.PagesDeploymentStatus{Status: string(status)})
}

// CancelPagesDeployment acknowledges cancellation and never rolls back the
// current content, because Pages keeps only one flat current state.
func CancelPagesDeployment(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/pages/deployments/{digest}/cancel repository repoCancelPagesDeployment
	// ---
	// summary: Stop polling a Pages deployment
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repository
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// - name: digest
	//   in: path
	//   description: SHA-256 digest returned when the deployment was created
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     description: Deployment polling was cancelled
	//   default:
	//     "$ref": "#/responses/error"
	if _, ok := getPagesActionsTask(ctx); !ok {
		return
	}
	ctx.Status(http.StatusNoContent)
}
