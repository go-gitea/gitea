// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/routers/repo"

	"code.gitea.io/git"
)

// GetRawFile get a file by path on a repository
func GetRawFile(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/raw/{filepath} repository repoGetRawFile
	// ---
	// summary: Get a file from a repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: filepath
	//   in: path
	//   description: filepath of the file to get
	//   type: string
	//   required: true
	// responses:
	//   200:
	//     description: success
	if ctx.Repo.Repository.IsEmpty {
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
func GetArchive(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/archive/{archive} repository repoGetArchive
	// ---
	// summary: Get an archive of a repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: archive
	//   in: path
	//   description: archive to download, consisting of a git reference and archive
	//   type: string
	//   required: true
	// responses:
	//   200:
	//     description: success
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
	// swagger:operation GET /repos/{owner}/{repo}/editorconfig/{filepath} repository repoGetEditorConfig
	// ---
	// summary: Get the EditorConfig definitions of a file in a repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: filepath
	//   in: path
	//   description: filepath of file to get
	//   type: string
	//   required: true
	// responses:
	//   200:
	//     description: success
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
