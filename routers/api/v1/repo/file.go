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
	"fmt"
	"os"
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

// Create a fle in a repository
func CreateFile(ctx *context.APIContext, opt api.CreateFileOptions) {
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
	if opt.Branch != "" {
		ctx.Repo.BranchName = opt.Branch
	}
	fmt.Println(ctx.Repo.BranchName)
	os.Exit(1)

	//file, err := models.CreateFile(ctx.User, owner, ctx.Repo, models.FileOptions{
	//	Message: opt.Message,
	//	Author: models.IdentityOptions{
	//		Name:  opt.Author.Name,
	//		Email: opt.Author.Email,
	//	},
	//	Committer: models.IdentityOptions{
	//		Name:  opt.Committer.Name,
	//		Email: opt.Committer.Email,
	//	},
	//	Content: opt.Content,
	//	Branch: opt.Branch,
	//	Path: ctx.Repo.TreePath,
	//})
	//
	//branchName := opt.Branch
	//treePath := cleanUploadFileName(ctx.Repo.TreePath)
	//if len(treePath) == 0 {
	//	ctx.Error(403, "", "File name is invalid")
	//	return
	//}
	//
	//if len(opt.TreePath) == 0 {
	//	ctx.Data["Err_TreePath"] = true
	//	ctx.RenderWithErr(ctx.Tr("repo.editor.filename_cannot_be_empty"), tplEditFile, &opt)
	//	return
	//}
	//
	//if oldBranchName != branchName {
	//	if _, err := ctx.Repo.Repository.GetBranch(branchName); err == nil {
	//		ctx.Data["Err_NewBranchName"] = true
	//		ctx.RenderWithErr(ctx.Tr("repo.editor.branch_already_exists", branchName), tplEditFile, &opt)
	//		return
	//	}
	//} else if !canCommit {
	//	ctx.Data["Err_NewBranchName"] = true
	//	ctx.Data["commit_choice"] = frmCommitChoiceNewBranch
	//	ctx.RenderWithErr(ctx.Tr("repo.editor.cannot_commit_to_protected_branch", branchName), tplEditFile, &opt)
	//	return
	//}
	//
	//var newTreePath string
	//for index, part := range treeNames {
	//	newTreePath = path.Join(newTreePath, part)
	//	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(newTreePath)
	//	if err != nil {
	//		if git.IsErrNotExist(err) {
	//			// Means there is no item with that name, so we're good
	//			break
	//		}
	//
	//		ctx.ServerError("Repo.Commit.GetTreeEntryByPath", err)
	//		return
	//	}
	//	if index != len(treeNames)-1 {
	//		if !entry.IsDir() {
	//			ctx.Data["Err_TreePath"] = true
	//			ctx.RenderWithErr(ctx.Tr("repo.editor.directory_is_a_file", part), tplEditFile, &opt)
	//			return
	//		}
	//	} else {
	//		if entry.IsLink() {
	//			ctx.Data["Err_TreePath"] = true
	//			ctx.RenderWithErr(ctx.Tr("repo.editor.file_is_a_symlink", part), tplEditFile, &opt)
	//			return
	//		}
	//		if entry.IsDir() {
	//			ctx.Data["Err_TreePath"] = true
	//			ctx.RenderWithErr(ctx.Tr("repo.editor.filename_is_a_directory", part), tplEditFile, &opt)
	//			return
	//		}
	//	}
	//}
	//
	//if !isNewFile {
	//	_, err := ctx.Repo.Commit.GetTreeEntryByPath(oldTreePath)
	//	if err != nil {
	//		if git.IsErrNotExist(err) {
	//			ctx.Data["Err_TreePath"] = true
	//			ctx.RenderWithErr(ctx.Tr("repo.editor.file_editing_no_longer_exists", oldTreePath), tplEditFile, &opt)
	//		} else {
	//			ctx.ServerError("GetTreeEntryByPath", err)
	//		}
	//		return
	//	}
	//	if lastCommit != ctx.Repo.CommitID {
	//		files, err := ctx.Repo.Commit.GetFilesChangedSinceCommit(lastCommit)
	//		if err != nil {
	//			ctx.ServerError("GetFilesChangedSinceCommit", err)
	//			return
	//		}
	//
	//		for _, file := range files {
	//			if file == opt.TreePath {
	//				ctx.RenderWithErr(ctx.Tr("repo.editor.file_changed_while_editing", ctx.Repo.RepoLink+"/compare/"+lastCommit+"..."+ctx.Repo.CommitID), tplEditFile, &opt)
	//				return
	//			}
	//		}
	//	}
	//}
	//
	//if oldTreePath != opt.TreePath {
	//	// We have a new filename (rename or completely new file) so we need to make sure it doesn't already exist, can't clobber.
	//	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(opt.TreePath)
	//	if err != nil {
	//		if !git.IsErrNotExist(err) {
	//			ctx.ServerError("GetTreeEntryByPath", err)
	//			return
	//		}
	//	}
	//	if entry != nil {
	//		ctx.Data["Err_TreePath"] = true
	//		ctx.RenderWithErr(ctx.Tr("repo.editor.file_already_exists", opt.TreePath), tplEditFile, &opt)
	//		return
	//	}
	//}
	//
	//message := strings.TrimSpace(opt.CommitSummary)
	//if len(message) == 0 {
	//	if isNewFile {
	//		message = ctx.Tr("repo.editor.add", opt.TreePath)
	//	} else {
	//		message = ctx.Tr("repo.editor.update", opt.TreePath)
	//	}
	//}
	//
	//opt.CommitMessage = strings.TrimSpace(opt.CommitMessage)
	//if len(opt.CommitMessage) > 0 {
	//	message += "\n\n" + opt.CommitMessage
	//}
	//
	//if err := uploader.UpdateRepoFile(ctx.Repo.Repository, ctx.User, &uploader.UpdateRepoFileOptions{
	//	LastCommitID: lastCommit,
	//	OldBranch:    oldBranchName,
	//	NewBranch:    branchName,
	//	OldTreeName:  oldTreePath,
	//	NewTreeName:  opt.TreePath,
	//	Message:      message,
	//	Content:      strings.Replace(opt.Content, "\r", "", -1),
	//	IsNewFile:    isNewFile,
	//}); err != nil {
	//	ctx.Data["Err_TreePath"] = true
	//	ctx.RenderWithErr(ctx.Tr("repo.editor.fail_to_update_file", opt.TreePath, err), tplEditFile, &opt)
	//	return
	//}
	//
	//ctx.Redirect(ctx.Repo.RepoLink + "/src/branch/" + branchName + "/" + strings.NewReplacer("%", "%25", "#", "%23", " ", "%20", "?", "%3F").Replace(opt.TreePath))
}

// Update a fle in a repository
func UpdateFile(ctx *context.APIContext, form api.UpdateFileOptions) {
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

// Create a fle in a repository
func DeleteFile(ctx *context.APIContext, form api.DeleteFileOptions) {
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
