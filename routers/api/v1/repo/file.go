// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/routers/repo"
	api "code.gitea.io/sdk/gitea"
	"path"
	"strings"
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



func renderCommitRights(ctx *context.APIContext) bool {
	canCommit, err := ctx.Repo.CanCommitToBranch(ctx.User)
	if err != nil {
		log.Error(4, "CanCommitToBranch: %v", err)
	}
	return canCommit
}

func cleanUploadFileName(name string) string {
	// Rebase the filename
	name = strings.Trim(path.Clean("/"+name), " /")
	// Git disallows any filenames to have a .git directory in them.
	for _, part := range strings.Split(name, "/") {
		if strings.ToLower(part) == ".git" {
			return ""
		}
	}
	return name
}

// Dummy function to for a Swagger definition of the Create File API request
func CreateFile(ctx *context.APIContext, opt api.CreateUpdateFileOptions) {
	// swagger:operation POST /repos/{owner}/{repo}/contents/{filepath} repository repoCreateFile
	// ---
	// summary: Create a file in a repository
	// consumes:
	// - application/json
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
	//   description: path of the file to create
	// - name: body
	//   in: body
	//   description: Both the `author` and `committer` parameters have the same keys; `sha` is the SHA for the file that already exists
	//   schema:
	//     "$ref": "#/definitions/CreateFileOptions"
	// responses:
	//   "201":
	//     "$ref": "#/responses/FileResponse"
	//owner := user.GetUserByParams(ctx)
	//canCommit := renderCommitRights(ctx)
}

// Dummy function to for a Swagger definition of the Update File API request
func UpdateFile(ctx *context.APIContext, form api.CreateUpdateFileOptions) {
	// swagger:operation PUT /repos/{owner}/{repo}/contents/{filepath} repository repoUpdateFile
	// ---
	// summary: Update a file in a repository
	// consumes:
	// - application/json
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
	//   description: path of the file to update
	// - name: body
	//   in: body
	//   description: Both the `author` and `committer` parameters have the same keys; `sha` is the SHA for the file that already exists
	//   schema:
	//     "$ref": "#/definitions/UpdateFileOptions"
	// responses:
	//   "201":
	//     "$ref": "#/responses/FileResponse"
	ctx.JSON(200, &api.FileResponse{})
}

// Handles if a API call is for Creating or Updating a file
func CreateUpdateFile(ctx *context.APIContext, opt api.CreateUpdateFileOptions) {
	opts := models.FileOptions{
		Content: opt.Content,
		Message: opt.Message,
		SHA: opt.SHA,
		OrigPath: opt.OrigPath,
		BranchName: opt.BranchName,
		NewBranchName: opt.NewBranchName,
		Committer: models.IdentityOptions{
			Name: opt.Committer.Name,
			Email: opt.Committer.Email,
		},
		Author: models.IdentityOptions{
			Name: opt.Author.Name,
			Email: opt.Author.Email,
		},
	}
	if opts.Committer.Name == "" || opts.Committer.Email == "" {
		if opts.Author.Name == "" || opts.Author.Email == "" {
			opts.Author.Name = ctx.User.Name
			opts.Author.Email = ctx.User.Email
		}
		opts.Committer = opts.Author
	}
	if opts.Author.Name == "" || opts.Author.Email == "" {
		opts.Author = opts.Committer
	}
	models.CreateOrUpdateFile(opts)
}

// Delete a fle in a repository
func DeleteFile(ctx *context.APIContext, opt api.DeleteFileOptions) {
	// swagger:operation DELETE /repos/{owner}/{repo}/contents/{filepath} repository repoDeleteFile
	// ---
	// summary: Delete a file in a repository
	// consumes:
	// - application/json
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
	//   description: path of the file to delete
	// - name: body
	//   in: body
	//   description: Both the `author` and `committer` parameters have the same keys; `sha` is the SHA for the file that already exists
	//   schema:
	//     "$ref": "#/definitions/DeleteFileOptions"
	// responses:
	//   "201":
	//     "$ref": "#/responses/FileDeleteResponse"
	ctx.JSON(200, &api.FileDeleteResponse{})
}
