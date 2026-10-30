// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"mime"
	"net/http"

	pages_model "gitea.dev/models/pages"
	pages_content "gitea.dev/modules/pages"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/routers/api/v1/utils"
	"gitea.dev/services/context"
	pages_service "gitea.dev/services/pages"
)

// UploadPagesContent saves the repository's current Pages artifact.
func UploadPagesContent(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/pages/content repository repoUploadPagesContent
	// ---
	// summary: Upload a Pages TAR.GZ artifact
	// consumes:
	// - application/gzip
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
	//   description: complete TAR.GZ bytes
	//   required: true
	//   schema:
	//     type: string
	//     format: binary
	// responses:
	//   "200":
	//     "$ref": "#/responses/PagesContentCommit"
	//   default:
	//     "$ref": "#/responses/error"
	contentType, _, err := mime.ParseMediaType(ctx.Req.Header.Get("Content-Type"))
	if err != nil || contentType != "application/gzip" || ctx.Req.Header.Get("Content-Encoding") != "" {
		ctx.APIError(http.StatusUnsupportedMediaType, "Pages content must be raw application/gzip")
		return
	}
	if ctx.Req.ContentLength > setting.Pages.MaxTarGzSize {
		ctx.APIError(http.StatusRequestEntityTooLarge, "Pages archive exceeds the compressed size limit")
		return
	}
	result, err := pages_service.CommitUploadContent(ctx, ctx.Repo.Repository.ID, ctx.Req.Body)
	if err != nil {
		switch {
		case errors.Is(err, pages_service.ErrSourceChanged):
			ctx.APIError(http.StatusConflict, "Pages state changed while uploading")
		case errors.Is(err, pages_content.ErrArchiveTooLarge):
			ctx.APIError(http.StatusRequestEntityTooLarge, "Pages archive exceeds a size limit")
		case errors.Is(err, pages_content.ErrInvalidArchive), errors.Is(err, pages_content.ErrArchivePath),
			errors.Is(err, pages_content.ErrArchiveEntry), errors.Is(err, pages_content.ErrArchiveSymlink):
			ctx.APIError(http.StatusBadRequest, "Invalid Pages archive")
		default:
			ctx.APIErrorInternal(err)
		}
		return
	}
	ctx.JSON(http.StatusOK, api.PagesContentCommit{
		Version: versionToAPI(result.Version, true), PagesUpdatedUnixNano: result.PagesUpdatedUnixNano,
	})
}

// DeletePagesVersion deletes one retained Pages version.
func DeletePagesVersion(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/pages/versions/{id} repository repoDeletePagesVersion
	// ---
	// summary: Delete a Pages version
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
	// - name: id
	//   in: path
	//   description: ID of the Pages version
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     description: Pages version deleted
	//   default:
	//     "$ref": "#/responses/error"
	err := pages_service.DeleteVersion(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("id"))
	if err != nil {
		switch {
		case errors.Is(err, util.ErrNotExist):
			ctx.APIErrorNotFound("PagesVersion")
		default:
			ctx.APIErrorInternal(err)
		}
		return
	}
	ctx.Status(http.StatusNoContent)
}

func versionToAPI(version *pages_model.Version, current bool) *api.PagesVersion {
	return &api.PagesVersion{
		ID: version.ID, TarGzSHA256: version.TarGzSHA256, OriginType: string(version.OriginType),
		OriginActionRunID: version.OriginActionRunID, CreatedUnix: int64(version.CreatedUnix), Current: current,
	}
}

// ListPagesVersions lists retained Pages versions for a repository.
func ListPagesVersions(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/pages/versions repository repoListPagesVersions
	// ---
	// summary: List Pages versions
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
	// - name: page
	//   in: query
	//   description: page number of results to return
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/PagesVersionList"
	//   default:
	//     "$ref": "#/responses/error"
	listOptions := utils.GetListOptions(ctx)
	versions, total, err := pages_model.FindVersions(ctx, &pages_model.FindVersionOptions{
		ListOptions: listOptions,
		RepoID:      ctx.Repo.Repository.ID,
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	site, has, err := pages_model.GetSiteByRepoID(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	currentVersionID := int64(0)
	if has {
		currentVersionID = site.CurrentVersionID
	}
	response := make([]*api.PagesVersion, 0, len(versions))
	for _, version := range versions {
		response = append(response, versionToAPI(version, version.ID == currentVersionID))
	}
	ctx.SetLinkHeader(total, listOptions.PageSize)
	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, response)
}

// RestorePagesVersion makes one retained version current again.
func RestorePagesVersion(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/pages/versions/{id}/restore repository repoRestorePagesVersion
	// ---
	// summary: Restore a Pages version
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
	// - name: id
	//   in: path
	//   description: ID of the Pages version
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/PagesContentCommit"
	//   default:
	//     "$ref": "#/responses/error"
	result, err := pages_service.RestoreVersion(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("id"))
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.APIErrorNotFound("PagesVersion")
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	ctx.JSON(http.StatusOK, api.PagesContentCommit{
		Version: versionToAPI(result.Version, true), PagesUpdatedUnixNano: result.PagesUpdatedUnixNano,
	})
}
