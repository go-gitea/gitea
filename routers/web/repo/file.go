// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/services/context"
	files_service "code.gitea.io/gitea/services/repository/files"
)

// canReadFiles returns true if repository is readable and user has proper access level.
func canReadFiles(r *context.Repository) bool {
	return r.Permission.CanRead(unit.TypeCode)
}

// GetContents Get the metadata and contents (if a file) of an entry in a repository, or a list of entries if a dir
func GetContents(ctx *context.Context) {
	if !canReadFiles(ctx.Repo) {
		ctx.NotFound("Invalid FilePath", nil)
		return
	}

	treePath := ctx.PathParam("*")
	ref := ctx.FormTrim("ref")

	if fileList, err := files_service.GetContentsOrList(ctx, ctx.Repo.Repository, treePath, ref); err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound("GetContentsOrList", err)
			return
		}
		ctx.ServerError("Repo.GitRepo.GetCommit", err)
	} else {
		ctx.JSON(http.StatusOK, fileList)
	}
}

// GetContentsList Get the metadata of all the entries of the root dir
func GetContentsList(ctx *context.Context) {
	GetContents(ctx)
}
