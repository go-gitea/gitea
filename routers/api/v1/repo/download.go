// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/services/context"
	archiver_service "code.gitea.io/gitea/services/repository/archiver"
)

func serveRepoArchive(ctx *context.APIContext, reqFileName string) {
	gitRepo, err := gitrepo.RepositoryFromRequestContextOrOpen(ctx, ctx.Repo.Repository)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	aReq, err := archiver_service.NewRequest(ctx.Repo.Repository.ID, ctx.Repo.GitRepo, reqFileName)
	if err != nil {
		if errors.Is(err, archiver_service.ErrUnknownArchiveFormat{}) {
			ctx.APIError(http.StatusBadRequest, err)
		} else if errors.Is(err, archiver_service.RepoRefNotFoundError{}) {
			ctx.APIError(http.StatusNotFound, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	archiver_service.ServeRepoArchive(ctx.Base, ctx.Repo.Repository, gitRepo, aReq)
}

func DownloadArchive(ctx *context.APIContext) {
	var tp git.ArchiveType
	switch ballType := ctx.PathParam("ball_type"); ballType {
	case "tarball":
		tp = git.ArchiveTarGz
	case "zipball":
		tp = git.ArchiveZip
	case "bundle":
		tp = git.ArchiveBundle
	default:
		ctx.APIError(http.StatusBadRequest, "Unknown archive type: "+ballType)
		return
	}
	serveRepoArchive(ctx, ctx.PathParam("*")+"."+tp.String())
}
