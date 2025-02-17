// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/services/context"
	archiver_service "code.gitea.io/gitea/services/repository/archiver"
)

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
		ctx.APIError(http.StatusBadRequest, fmt.Sprintf("Unknown archive type: %s", ballType))
		return
	}

	if ctx.Repo.GitRepo == nil {
		var err error
		ctx.Repo.GitRepo, err = gitrepo.RepositoryFromRequestContextOrOpen(ctx, ctx.Repo.Repository)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	}

	r, err := archiver_service.NewRequest(ctx.Repo.Repository.ID, ctx.Repo.GitRepo, ctx.PathParam("*")+"."+tp.String())
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	archive, err := r.Await(ctx)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	download(ctx, r.GetArchiveName(), archive)
}
