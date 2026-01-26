// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
	archiver_service "code.gitea.io/gitea/services/repository/archiver"
)

func serveRepoArchive(ctx *context.APIContext, reqFileName string, paths []string) {
	aReq, err := archiver_service.NewRequest(ctx.Repo.Repository, ctx.Repo.GitRepo, reqFileName, paths)
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.APIError(http.StatusBadRequest, err)
		} else if errors.Is(err, util.ErrNotExist) {
			ctx.APIError(http.StatusNotFound, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	err = archiver_service.ServeRepoArchive(ctx.Base, aReq)
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.APIError(http.StatusBadRequest, err)
		} else {
			ctx.APIErrorInternal(err)
		}
	}
}

// DownloadArchive is the GitHub-compatible endpoint to download repository archives
// TODO: The API document is missing: Add github compatible tarball download API endpoints (#32572)
func DownloadArchive(ctx *context.APIContext) {
	var tp repo_model.ArchiveType
	switch ballType := ctx.PathParam("ball_type"); ballType {
	case "tarball":
		tp = repo_model.ArchiveTarGz
	case "zipball":
		tp = repo_model.ArchiveZip
	case "bundle":
		tp = repo_model.ArchiveBundle
	default:
		ctx.APIError(http.StatusBadRequest, "Unknown archive type: "+ballType)
		return
	}
	serveRepoArchive(ctx, ctx.PathParam("*")+"."+tp.String(), ctx.FormStrings("path"))
}
