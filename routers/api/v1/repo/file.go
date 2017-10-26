// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/git"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/routers/repo"
)

// GetRawFile get a file by path on a repository
// see https://github.com/gogits/go-gogs-client/wiki/Repositories-Contents#download-raw-content
func GetRawFile(ctx *context.APIContext) {
	if !ctx.Repo.HasAccess() {
		ctx.Status(404)
		return
	}

	if ctx.Repo.Repository.IsBare {
		ctx.Status(404)
		return
	}

	blob, err := ctx.Repo.Commit.GetBlobByPath(ctx.Repo.TreePath)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetBlobByPath", err)
		}
		return
	}
	if err = repo.ServeBlob(ctx.Context, blob); err != nil {
		ctx.Error(500, "ServeBlob", err)
	}
}

// GetArchive get archive of a repository
// see https://github.com/gogits/go-gogs-client/wiki/Repositories-Contents#download-archive
func GetArchive(ctx *context.APIContext) {
	repoPath := models.RepoPath(ctx.Params(":username"), ctx.Params(":reponame"))
	gitRepo, err := git.OpenRepository(repoPath)
	if err != nil {
		ctx.Error(500, "OpenRepository", err)
		return
	}
	ctx.Repo.GitRepo = gitRepo

	repo.Download(ctx.Context)
}

// GetEditorconfig get editor config of a repository
func GetEditorconfig(ctx *context.APIContext) {
	ec, err := ctx.Repo.GetEditorconfig()
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.Error(404, "GetEditorconfig", err)
		} else {
			ctx.Error(500, "GetEditorconfig", err)
		}
		return
	}

	fileName := ctx.Params("filename")
	def := ec.GetDefinitionForFilename(fileName)
	if def == nil {
		ctx.Error(404, "GetDefinitionForFilename", err)
		return
	}
	ctx.JSON(200, def)
}
