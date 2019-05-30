// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"encoding/base64"
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/repofiles"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/repo"
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
		ctx.NotFound()
		return
	}

	blob, err := ctx.Repo.Commit.GetBlobByPath(ctx.Repo.TreePath)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetBlobByPath", err)
		}
		return
	}
	if err = repo.ServeBlob(ctx.Context, blob); err != nil {
		ctx.Error(http.StatusInternalServerError, "ServeBlob", err)
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
		ctx.Error(http.StatusInternalServerError, "OpenRepository", err)
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
			ctx.NotFound(err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetEditorconfig", err)
		}
		return
	}

	fileName := ctx.Params("filename")
	def := ec.GetDefinitionForFilename(fileName)
	if def == nil {
		ctx.NotFound(err)
		return
	}
	ctx.JSON(http.StatusOK, def)
}

// CanWriteFiles returns true if repository is editable and user has proper access level.
func CanWriteFiles(r *context.Repository) bool {
	return r.Permission.CanWrite(models.UnitTypeCode) && !r.Repository.IsMirror && !r.Repository.IsArchived
}

// CanReadFiles returns true if repository is readable and user has proper access level.
func CanReadFiles(r *context.Repository) bool {
	return r.Permission.CanRead(models.UnitTypeCode)
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
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   description: "'content' must be base64 encoded\n\n 'author' and 'committer' are optional (if only one is given, it will be used for the other, otherwise the authenticated user will be used)\n\n If 'branch' is not given, default branch will be used\n\n 'sha' is the SHA for the file that already exists\n\n 'new_branch' (optional) will make a new branch from 'branch' before creating the file"
	//   schema:
	//     "$ref": "#/definitions/CreateFileOptions"
	// responses:
	//   "201":
	//     "$ref": "#/responses/FileResponse"

	opts := &repofiles.UpdateRepoFileOptions{
		Content:   apiOpts.Content,
		IsNewFile: true,
		Message:   apiOpts.Message,
		TreePath:  ctx.Params("*"),
		OldBranch: apiOpts.BranchName,
		NewBranch: apiOpts.NewBranchName,
		Committer: &repofiles.IdentityOptions{
			Name:  apiOpts.Committer.Name,
			Email: apiOpts.Committer.Email,
		},
		Author: &repofiles.IdentityOptions{
			Name:  apiOpts.Author.Name,
			Email: apiOpts.Author.Email,
		},
	}
	if fileResponse, err := createOrUpdateFile(ctx, opts); err != nil {
		ctx.Error(http.StatusInternalServerError, "CreateFile", err)
	} else {
		ctx.JSON(http.StatusCreated, fileResponse)
	}
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
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   description: "'content' must be base64 encoded\n\n 'sha' is the SHA for the file that already exists\n\n 'author' and 'committer' are optional (if only one is given, it will be used for the other, otherwise the authenticated user will be used)\n\n If 'branch' is not given, default branch will be used\n\n 'new_branch' (optional) will make a new branch from 'branch' before updating the file"
	//   schema:
	//     "$ref": "#/definitions/UpdateFileOptions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/FileResponse"

	opts := &repofiles.UpdateRepoFileOptions{
		Content:      apiOpts.Content,
		SHA:          apiOpts.SHA,
		IsNewFile:    false,
		Message:      apiOpts.Message,
		FromTreePath: apiOpts.FromPath,
		TreePath:     ctx.Params("*"),
		OldBranch:    apiOpts.BranchName,
		NewBranch:    apiOpts.NewBranchName,
		Committer: &repofiles.IdentityOptions{
			Name:  apiOpts.Committer.Name,
			Email: apiOpts.Committer.Email,
		},
		Author: &repofiles.IdentityOptions{
			Name:  apiOpts.Author.Name,
			Email: apiOpts.Author.Email,
		},
	}

	if fileResponse, err := createOrUpdateFile(ctx, opts); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateFile", err)
	} else {
		ctx.JSON(http.StatusOK, fileResponse)
	}
}

// Called from both CreateFile or UpdateFile to handle both
func createOrUpdateFile(ctx *context.APIContext, opts *repofiles.UpdateRepoFileOptions) (*api.FileResponse, error) {
	if !CanWriteFiles(ctx.Repo) {
		return nil, models.ErrUserDoesNotHaveAccessToRepo{
			UserID:   ctx.User.ID,
			RepoName: ctx.Repo.Repository.LowerName,
		}
	}

	content, err := base64.StdEncoding.DecodeString(opts.Content)
	if err != nil {
		return nil, err
	}
	opts.Content = string(content)

	return repofiles.CreateOrUpdateRepoFile(ctx.Repo.Repository, ctx.User, opts)
}

// DeleteFile Delete a fle in a repository
func DeleteFile(ctx *context.APIContext, apiOpts api.DeleteFileOptions) {
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
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   description: "'sha' is the SHA for the file to be deleted\n\n 'author' and 'committer' are optional (if only one is given, it will be used for the other, otherwise the authenticated user will be used)\n\n If 'branch' is not given, default branch will be used\n\n 'new_branch' (optional) will make a new branch from 'branch' before deleting the file"
	//   schema:
	//     "$ref": "#/definitions/DeleteFileOptions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/FileDeleteResponse"
	if !CanWriteFiles(ctx.Repo) {
		ctx.Error(http.StatusInternalServerError, "DeleteFile", models.ErrUserDoesNotHaveAccessToRepo{
			UserID:   ctx.User.ID,
			RepoName: ctx.Repo.Repository.LowerName,
		})
		return
	}

	opts := &repofiles.DeleteRepoFileOptions{
		Message:   apiOpts.Message,
		OldBranch: apiOpts.BranchName,
		NewBranch: apiOpts.NewBranchName,
		SHA:       apiOpts.SHA,
		TreePath:  ctx.Params("*"),
		Committer: &repofiles.IdentityOptions{
			Name:  apiOpts.Committer.Name,
			Email: apiOpts.Committer.Email,
		},
		Author: &repofiles.IdentityOptions{
			Name:  apiOpts.Author.Name,
			Email: apiOpts.Author.Email,
		},
	}

	if fileResponse, err := repofiles.DeleteRepoFile(ctx.Repo.Repository, ctx.User, opts); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteFile", err)
	} else {
		ctx.JSON(http.StatusOK, fileResponse)
	}
}

// GetFileContents Get the contents of a fle in a repository
func GetFileContents(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/contents/{filepath} repository repoGetFileContents
	// ---
	// summary: Gets the contents of a file or directory in a repository
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
	//   type: string
	//   required: true
	// - name: ref
	//   in: query
	//   description: "The name of the commit/branch/tag. Default the repository’s default branch (usually master)"
	//   required: false
	//   type: string
	// responses:
	//   "200":
	//     "$ref": "#/responses/FileContentResponse"

	if !CanReadFiles(ctx.Repo) {
		ctx.Error(http.StatusInternalServerError, "GetFileContents", models.ErrUserDoesNotHaveAccessToRepo{
			UserID:   ctx.User.ID,
			RepoName: ctx.Repo.Repository.LowerName,
		})
		return
	}

	treePath := ctx.Params("*")
	ref := ctx.QueryTrim("ref")

	if fileContents, err := repofiles.GetFileContents(ctx.Repo.Repository, treePath, ref); err != nil {
		ctx.Error(http.StatusInternalServerError, "GetFileContents", err)
	} else {
		ctx.JSON(http.StatusOK, fileContents)
	}
}
