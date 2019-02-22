// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/uploader"
	"code.gitea.io/gitea/routers/repo"
	api "code.gitea.io/sdk/gitea"
	"encoding/base64"
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

// CanWriteFiles returns true if repository is editable and user has proper access level.
func CanWriteFiles(r *context.Repository) bool {
	return r.Permission.CanWrite(models.UnitTypeCode) && !r.Repository.IsMirror && !r.Repository.IsArchived
}

// CreateFile handles API call for creating a file
func CreateFile(ctx *context.APIContext, apiOpts api.CreateFileOptions) {
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
	opts := uploader.UpdateRepoFileOptions{
		Content:   apiOpts.Content,
		IsNewFile: true,
		Message:   apiOpts.Message,
		TreeName:  ctx.Repo.TreePath,
		OldBranch: apiOpts.BranchName,
		NewBranch: apiOpts.NewBranchName,
		Committer: &uploader.IdentityOptions{
			Name:  apiOpts.Committer.Name,
			Email: apiOpts.Committer.Email,
		},
		Author: &uploader.IdentityOptions{
			Name:  apiOpts.Author.Name,
			Email: apiOpts.Author.Email,
		},
	}
	createOrUpdateFile(ctx, &opts)
}

// UpdateFile handles API call for updating a file
func UpdateFile(ctx *context.APIContext, apiOpts api.UpdateFileOptions) {
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
	opts := uploader.UpdateRepoFileOptions{
		Content:      apiOpts.Content,
		SHA:          apiOpts.SHA,
		IsNewFile:    false,
		Message:      apiOpts.Message,
		FromTreeName: apiOpts.FromPath,
		TreeName:     ctx.Repo.TreePath,
		OldBranch:    apiOpts.BranchName,
		NewBranch:    apiOpts.NewBranchName,
		Committer: &uploader.IdentityOptions{
			Name:  apiOpts.Committer.Name,
			Email: apiOpts.Committer.Email,
		},
		Author: &uploader.IdentityOptions{
			Name:  apiOpts.Author.Name,
			Email: apiOpts.Author.Email,
		},
	}

	createOrUpdateFile(ctx, &opts)

	ctx.JSON(200, &api.FileResponse{})
}

// Handles if an API call is for updating a repo file
func createOrUpdateFile(ctx *context.APIContext, opts *uploader.UpdateRepoFileOptions) {
	if !CanWriteFiles(ctx.Repo) {
		ctx.Error(500, "", models.ErrUserDoesNotHaveAccessToRepo{ctx.User.ID, ctx.Repo.Repository.LowerName})
		return
	}

	if content, err := base64.StdEncoding.DecodeString(opts.Content); err != nil {
		ctx.Error(500, "", err)
		return
	} else {
		opts.Content = string(content)
	}

	if file, err := uploader.CreateOrUpdateRepoFile(ctx.Repo.Repository, ctx.Repo.GitRepo, ctx.User, opts); err != nil {
		ctx.Error(500, "", err)
	} else {
		ctx.JSON(200, file)
	}
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
