// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"mime"
	"net/http"

	pages_content "gitea.dev/modules/pages"
	api "gitea.dev/modules/structs"
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
	if ctx.Req.ContentLength > pages_content.DefaultMaxCompressedSize {
		ctx.APIError(http.StatusRequestEntityTooLarge, "Pages archive exceeds the compressed size limit")
		return
	}
	result, err := pages_service.CommitUploadContent(ctx, ctx.Repo.Repository.ID, ctx.Req.Body)
	if err != nil {
		switch {
		case errors.Is(err, pages_service.ErrSourceChanged):
			ctx.APIError(http.StatusConflict, "Pages upload is not the current source")
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
		TarGzSHA256: result.TarGzSHA256, PagesUpdatedUnixNano: result.PagesUpdatedUnixNano,
	})
}
