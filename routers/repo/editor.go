// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"strings"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
)

const (
	tplEditFile        base.TplName = "repo/editor/edit"
	tplEditDiffPreview base.TplName = "repo/editor/diff_preview"
	tplDeleteFile      base.TplName = "repo/editor/delete"
	tplUploadFile      base.TplName = "repo/editor/upload"

	frmCommitChoiceDirect    string = "direct"
	frmCommitChoiceNewBranch string = "commit-to-new-branch"
)

func renderCommitRights(ctx *context.Context) bool {
	canCommit, err := ctx.Repo.CanCommitToBranch(ctx.User)
	if err != nil {
		log.Error(4, "CanCommitToBranch: %v", err)
	}
	ctx.Data["CanCommitToBranch"] = canCommit
	return canCommit
}

// getParentTreeFields returns list of parent tree names and corresponding tree paths
// based on given tree path.
func getParentTreeFields(treePath string) (treeNames []string, treePaths []string) {
	if len(treePath) == 0 {
		return treeNames, treePaths
	}

	treeNames = strings.Split(treePath, "/")
	treePaths = make([]string, len(treeNames))
	for i := range treeNames {
		treePaths[i] = strings.Join(treeNames[:i+1], "/")
	}
	return treeNames, treePaths
}

func editFile(ctx *context.Context, isNewFile bool) {
	ctx.Data["PageIsEdit"] = true
	ctx.Data["IsNewFile"] = isNewFile
	ctx.Data["RequireHighlightJS"] = true
	ctx.Data["RequireSimpleMDE"] = true
	canCommit := renderCommitRights(ctx)

	treeNames, treePaths := getParentTreeFields(ctx.Repo.TreePath)

	if !isNewFile {
		entry, err := ctx.Repo.Commit.GetTreeEntryByPath(ctx.Repo.TreePath)
		if err != nil {
			ctx.NotFoundOrServerError("GetTreeEntryByPath", git.IsErrNotExist, err)
			return
		}

		// No way to edit a directory online.
		if entry.IsDir() {
			ctx.Handle(404, "", nil)
			return
		}

		blob := entry.Blob()
		dataRc, err := blob.Data()
		if err != nil {
			ctx.Handle(404, "blob.Data", err)
			return
		}

		ctx.Data["FileSize"] = blob.Size()
		ctx.Data["FileName"] = blob.Name()

		buf := make([]byte, 1024)
		n, _ := dataRc.Read(buf)
		buf = buf[:n]

		// Only text file are editable online.
		if !base.IsTextFile(buf) {
			ctx.Handle(404, "", nil)
			return
		}

		d, _ := ioutil.ReadAll(dataRc)
		buf = append(buf, d...)
		if content, err := templates.ToUTF8WithErr(buf); err != nil {
			if err != nil {
				log.Error(4, "ToUTF8WithErr: %v", err)
			}
			ctx.Data["FileContent"] = string(buf)
		} else {
			ctx.Data["FileContent"] = content
		}
	} else {
		treeNames = append(treeNames, "") // Append empty string to allow user name the new file.
	}

	ctx.Data["TreeNames"] = treeNames
	ctx.Data["TreePaths"] = treePaths
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchName
	ctx.Data["commit_summary"] = ""
	ctx.Data["commit_message"] = ""
	if canCommit {
		ctx.Data["commit_choice"] = frmCommitChoiceDirect
	} else {
		ctx.Data["commit_choice"] = frmCommitChoiceNewBranch
	}
	ctx.Data["new_branch_name"] = ""
	ctx.Data["last_commit"] = ctx.Repo.Commit.ID
	ctx.Data["MarkdownFileExts"] = strings.Join(setting.Markdown.FileExtensions, ",")
	ctx.Data["LineWrapExtensions"] = strings.Join(setting.Repository.Editor.LineWrapExtensions, ",")
	ctx.Data["PreviewableFileModes"] = strings.Join(setting.Repository.Editor.PreviewableFileModes, ",")
	ctx.Data["EditorconfigURLPrefix"] = fmt.Sprintf("%s/api/v1/repos/%s/editorconfig/", setting.AppSubURL, ctx.Repo.Repository.FullName())

	ctx.HTML(200, tplEditFile)
}

// EditFile render edit file page
func EditFile(ctx *context.Context) {
	editFile(ctx, false)
}

// NewFile render create file page
func NewFile(ctx *context.Context) {
	editFile(ctx, true)
}

func editFilePost(ctx *context.Context, form auth.EditRepoFileForm, isNewFile bool) {
	ctx.Data["PageIsEdit"] = true
	ctx.Data["IsNewFile"] = isNewFile
	ctx.Data["RequireHighlightJS"] = true
	ctx.Data["RequireSimpleMDE"] = true
	canCommit := renderCommitRights(ctx)

	oldBranchName := ctx.Repo.BranchName
	branchName := oldBranchName
	oldTreePath := ctx.Repo.TreePath
	lastCommit := form.LastCommit
	form.LastCommit = ctx.Repo.Commit.ID.String()

	if form.CommitChoice == frmCommitChoiceNewBranch {
		branchName = form.NewBranchName
	}

	form.TreePath = strings.Trim(form.TreePath, " /")
	treeNames, treePaths := getParentTreeFields(form.TreePath)

	ctx.Data["TreePath"] = form.TreePath
	ctx.Data["TreeNames"] = treeNames
	ctx.Data["TreePaths"] = treePaths
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/" + branchName
	ctx.Data["FileContent"] = form.Content
	ctx.Data["commit_summary"] = form.CommitSummary
	ctx.Data["commit_message"] = form.CommitMessage
	ctx.Data["commit_choice"] = form.CommitChoice
	ctx.Data["new_branch_name"] = branchName
	ctx.Data["last_commit"] = form.LastCommit
	ctx.Data["MarkdownFileExts"] = strings.Join(setting.Markdown.FileExtensions, ",")
	ctx.Data["LineWrapExtensions"] = strings.Join(setting.Repository.Editor.LineWrapExtensions, ",")
	ctx.Data["PreviewableFileModes"] = strings.Join(setting.Repository.Editor.PreviewableFileModes, ",")

	if ctx.HasError() {
		ctx.HTML(200, tplEditFile)
		return
	}

	if len(form.TreePath) == 0 {
		ctx.Data["Err_TreePath"] = true
		ctx.RenderWithErr(ctx.Tr("repo.editor.filename_cannot_be_empty"), tplEditFile, &form)
		return
	}

	if oldBranchName != branchName {
		if _, err := ctx.Repo.Repository.GetBranch(branchName); err == nil {
			ctx.Data["Err_NewBranchName"] = true
			ctx.RenderWithErr(ctx.Tr("repo.editor.branch_already_exists", branchName), tplEditFile, &form)
			return
		}
	} else if !canCommit {
		ctx.Data["Err_NewBranchName"] = true
		ctx.Data["commit_choice"] = frmCommitChoiceNewBranch
		ctx.RenderWithErr(ctx.Tr("repo.editor.cannot_commit_to_protected_branch", branchName), tplEditFile, &form)
		return
	}

	var newTreePath string
	for index, part := range treeNames {
		newTreePath = path.Join(newTreePath, part)
		entry, err := ctx.Repo.Commit.GetTreeEntryByPath(newTreePath)
		if err != nil {
			if git.IsErrNotExist(err) {
				// Means there is no item with that name, so we're good
				break
			}

			ctx.Handle(500, "Repo.Commit.GetTreeEntryByPath", err)
			return
		}
		if index != len(treeNames)-1 {
			if !entry.IsDir() {
				ctx.Data["Err_TreePath"] = true
				ctx.RenderWithErr(ctx.Tr("repo.editor.directory_is_a_file", part), tplEditFile, &form)
				return
			}
		} else {
			if entry.IsLink() {
				ctx.Data["Err_TreePath"] = true
				ctx.RenderWithErr(ctx.Tr("repo.editor.file_is_a_symlink", part), tplEditFile, &form)
				return
			}
			if entry.IsDir() {
				ctx.Data["Err_TreePath"] = true
				ctx.RenderWithErr(ctx.Tr("repo.editor.filename_is_a_directory", part), tplEditFile, &form)
				return
			}
		}
	}

	if !isNewFile {
		_, err := ctx.Repo.Commit.GetTreeEntryByPath(oldTreePath)
		if err != nil {
			if git.IsErrNotExist(err) {
				ctx.Data["Err_TreePath"] = true
				ctx.RenderWithErr(ctx.Tr("repo.editor.file_editing_no_longer_exists", oldTreePath), tplEditFile, &form)
			} else {
				ctx.Handle(500, "GetTreeEntryByPath", err)
			}
			return
		}
		if lastCommit != ctx.Repo.CommitID {
			files, err := ctx.Repo.Commit.GetFilesChangedSinceCommit(lastCommit)
			if err != nil {
				ctx.Handle(500, "GetFilesChangedSinceCommit", err)
				return
			}

			for _, file := range files {
				if file == form.TreePath {
					ctx.RenderWithErr(ctx.Tr("repo.editor.file_changed_while_editing", ctx.Repo.RepoLink+"/compare/"+lastCommit+"..."+ctx.Repo.CommitID), tplEditFile, &form)
					return
				}
			}
		}
	}

	if oldTreePath != form.TreePath {
		// We have a new filename (rename or completely new file) so we need to make sure it doesn't already exist, can't clobber.
		entry, err := ctx.Repo.Commit.GetTreeEntryByPath(form.TreePath)
		if err != nil {
			if !git.IsErrNotExist(err) {
				ctx.Handle(500, "GetTreeEntryByPath", err)
				return
			}
		}
		if entry != nil {
			ctx.Data["Err_TreePath"] = true
			ctx.RenderWithErr(ctx.Tr("repo.editor.file_already_exists", form.TreePath), tplEditFile, &form)
			return
		}
	}

	message := strings.TrimSpace(form.CommitSummary)
	if len(message) == 0 {
		if isNewFile {
			message = ctx.Tr("repo.editor.add", form.TreePath)
		} else {
			message = ctx.Tr("repo.editor.update", form.TreePath)
		}
	}

	form.CommitMessage = strings.TrimSpace(form.CommitMessage)
	if len(form.CommitMessage) > 0 {
		message += "\n\n" + form.CommitMessage
	}

	if err := ctx.Repo.Repository.UpdateRepoFile(ctx.User, models.UpdateRepoFileOptions{
		LastCommitID: lastCommit,
		OldBranch:    oldBranchName,
		NewBranch:    branchName,
		OldTreeName:  oldTreePath,
		NewTreeName:  form.TreePath,
		Message:      message,
		Content:      strings.Replace(form.Content, "\r", "", -1),
		IsNewFile:    isNewFile,
	}); err != nil {
		ctx.Data["Err_TreePath"] = true
		ctx.RenderWithErr(ctx.Tr("repo.editor.fail_to_update_file", form.TreePath, err), tplEditFile, &form)
		return
	}

	ctx.Redirect(ctx.Repo.RepoLink + "/src/" + branchName + "/" + strings.NewReplacer("%", "%25", "#", "%23", " ", "%20", "?", "%3F").Replace(form.TreePath))
}

// EditFilePost response for editing file
func EditFilePost(ctx *context.Context, form auth.EditRepoFileForm) {
	editFilePost(ctx, form, false)
}

// NewFilePost response for creating file
func NewFilePost(ctx *context.Context, form auth.EditRepoFileForm) {
	editFilePost(ctx, form, true)
}

// DiffPreviewPost render preview diff page
func DiffPreviewPost(ctx *context.Context, form auth.EditPreviewDiffForm) {
	treePath := ctx.Repo.TreePath

	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(treePath)
	if err != nil {
		ctx.Error(500, "GetTreeEntryByPath: "+err.Error())
		return
	} else if entry.IsDir() {
		ctx.Error(422)
		return
	}

	diff, err := ctx.Repo.Repository.GetDiffPreview(ctx.Repo.BranchName, treePath, form.Content)
	if err != nil {
		ctx.Error(500, "GetDiffPreview: "+err.Error())
		return
	}

	if diff.NumFiles() == 0 {
		ctx.PlainText(200, []byte(ctx.Tr("repo.editor.no_changes_to_show")))
		return
	}
	ctx.Data["File"] = diff.Files[0]

	ctx.HTML(200, tplEditDiffPreview)
}

// DeleteFile render delete file page
func DeleteFile(ctx *context.Context) {
	ctx.Data["PageIsDelete"] = true
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchName
	ctx.Data["TreePath"] = ctx.Repo.TreePath
	canCommit := renderCommitRights(ctx)

	ctx.Data["commit_summary"] = ""
	ctx.Data["commit_message"] = ""
	if canCommit {
		ctx.Data["commit_choice"] = frmCommitChoiceDirect
	} else {
		ctx.Data["commit_choice"] = frmCommitChoiceNewBranch
	}
	ctx.Data["new_branch_name"] = ""

	ctx.HTML(200, tplDeleteFile)
}

// DeleteFilePost response for deleting file
func DeleteFilePost(ctx *context.Context, form auth.DeleteRepoFileForm) {
	ctx.Data["PageIsDelete"] = true
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchName
	ctx.Data["TreePath"] = ctx.Repo.TreePath
	canCommit := renderCommitRights(ctx)

	oldBranchName := ctx.Repo.BranchName
	branchName := oldBranchName

	if form.CommitChoice == frmCommitChoiceNewBranch {
		branchName = form.NewBranchName
	}
	ctx.Data["commit_summary"] = form.CommitSummary
	ctx.Data["commit_message"] = form.CommitMessage
	ctx.Data["commit_choice"] = form.CommitChoice
	ctx.Data["new_branch_name"] = branchName

	if ctx.HasError() {
		ctx.HTML(200, tplDeleteFile)
		return
	}

	if oldBranchName != branchName {
		if _, err := ctx.Repo.Repository.GetBranch(branchName); err == nil {
			ctx.Data["Err_NewBranchName"] = true
			ctx.RenderWithErr(ctx.Tr("repo.editor.branch_already_exists", branchName), tplDeleteFile, &form)
			return
		}
	} else if !canCommit {
		ctx.Data["Err_NewBranchName"] = true
		ctx.Data["commit_choice"] = frmCommitChoiceNewBranch
		ctx.RenderWithErr(ctx.Tr("repo.editor.cannot_commit_to_protected_branch", branchName), tplDeleteFile, &form)
		return
	}

	message := strings.TrimSpace(form.CommitSummary)
	if len(message) == 0 {
		message = ctx.Tr("repo.editor.delete", ctx.Repo.TreePath)
	}

	form.CommitMessage = strings.TrimSpace(form.CommitMessage)
	if len(form.CommitMessage) > 0 {
		message += "\n\n" + form.CommitMessage
	}

	if err := ctx.Repo.Repository.DeleteRepoFile(ctx.User, models.DeleteRepoFileOptions{
		LastCommitID: ctx.Repo.CommitID,
		OldBranch:    oldBranchName,
		NewBranch:    branchName,
		TreePath:     ctx.Repo.TreePath,
		Message:      message,
	}); err != nil {
		ctx.Handle(500, "DeleteRepoFile", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.editor.file_delete_success", ctx.Repo.TreePath))
	ctx.Redirect(ctx.Repo.RepoLink + "/src/" + branchName)
}

func renderUploadSettings(ctx *context.Context) {
	ctx.Data["RequireDropzone"] = true
	ctx.Data["UploadAllowedTypes"] = strings.Join(setting.Repository.Upload.AllowedTypes, ",")
	ctx.Data["UploadMaxSize"] = setting.Repository.Upload.FileMaxSize
	ctx.Data["UploadMaxFiles"] = setting.Repository.Upload.MaxFiles
}

// UploadFile render upload file page
func UploadFile(ctx *context.Context) {
	ctx.Data["PageIsUpload"] = true
	renderUploadSettings(ctx)
	canCommit := renderCommitRights(ctx)

	treeNames, treePaths := getParentTreeFields(ctx.Repo.TreePath)
	if len(treeNames) == 0 {
		// We must at least have one element for user to input.
		treeNames = []string{""}
	}

	ctx.Data["TreeNames"] = treeNames
	ctx.Data["TreePaths"] = treePaths
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchName
	ctx.Data["commit_summary"] = ""
	ctx.Data["commit_message"] = ""
	if canCommit {
		ctx.Data["commit_choice"] = frmCommitChoiceDirect
	} else {
		ctx.Data["commit_choice"] = frmCommitChoiceNewBranch
	}
	ctx.Data["new_branch_name"] = ""

	ctx.HTML(200, tplUploadFile)
}

// UploadFilePost response for uploading file
func UploadFilePost(ctx *context.Context, form auth.UploadRepoFileForm) {
	ctx.Data["PageIsUpload"] = true
	renderUploadSettings(ctx)
	canCommit := renderCommitRights(ctx)

	oldBranchName := ctx.Repo.BranchName
	branchName := oldBranchName

	if form.CommitChoice == frmCommitChoiceNewBranch {
		branchName = form.NewBranchName
	}

	form.TreePath = strings.Trim(form.TreePath, " /")
	treeNames, treePaths := getParentTreeFields(form.TreePath)
	if len(treeNames) == 0 {
		// We must at least have one element for user to input.
		treeNames = []string{""}
	}

	ctx.Data["TreePath"] = form.TreePath
	ctx.Data["TreeNames"] = treeNames
	ctx.Data["TreePaths"] = treePaths
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/" + branchName
	ctx.Data["commit_summary"] = form.CommitSummary
	ctx.Data["commit_message"] = form.CommitMessage
	ctx.Data["commit_choice"] = form.CommitChoice
	ctx.Data["new_branch_name"] = branchName

	if ctx.HasError() {
		ctx.HTML(200, tplUploadFile)
		return
	}

	if oldBranchName != branchName {
		if _, err := ctx.Repo.Repository.GetBranch(branchName); err == nil {
			ctx.Data["Err_NewBranchName"] = true
			ctx.RenderWithErr(ctx.Tr("repo.editor.branch_already_exists", branchName), tplUploadFile, &form)
			return
		}
	} else if !canCommit {
		ctx.Data["Err_NewBranchName"] = true
		ctx.Data["commit_choice"] = frmCommitChoiceNewBranch
		ctx.RenderWithErr(ctx.Tr("repo.editor.cannot_commit_to_protected_branch", branchName), tplUploadFile, &form)
		return
	}

	var newTreePath string
	for _, part := range treeNames {
		newTreePath = path.Join(newTreePath, part)
		entry, err := ctx.Repo.Commit.GetTreeEntryByPath(newTreePath)
		if err != nil {
			if git.IsErrNotExist(err) {
				// Means there is no item with that name, so we're good
				break
			}

			ctx.Handle(500, "Repo.Commit.GetTreeEntryByPath", err)
			return
		}

		// User can only upload files to a directory.
		if !entry.IsDir() {
			ctx.Data["Err_TreePath"] = true
			ctx.RenderWithErr(ctx.Tr("repo.editor.directory_is_a_file", part), tplUploadFile, &form)
			return
		}
	}

	message := strings.TrimSpace(form.CommitSummary)
	if len(message) == 0 {
		message = ctx.Tr("repo.editor.upload_files_to_dir", form.TreePath)
	}

	form.CommitMessage = strings.TrimSpace(form.CommitMessage)
	if len(form.CommitMessage) > 0 {
		message += "\n\n" + form.CommitMessage
	}

	if err := ctx.Repo.Repository.UploadRepoFiles(ctx.User, models.UploadRepoFileOptions{
		LastCommitID: ctx.Repo.CommitID,
		OldBranch:    oldBranchName,
		NewBranch:    branchName,
		TreePath:     form.TreePath,
		Message:      message,
		Files:        form.Files,
	}); err != nil {
		ctx.Data["Err_TreePath"] = true
		ctx.RenderWithErr(ctx.Tr("repo.editor.unable_to_upload_files", form.TreePath, err), tplUploadFile, &form)
		return
	}

	ctx.Redirect(ctx.Repo.RepoLink + "/src/" + branchName + "/" + form.TreePath)
}

// UploadFileToServer upload file to server file dir not git
func UploadFileToServer(ctx *context.Context) {
	file, header, err := ctx.Req.FormFile("file")
	if err != nil {
		ctx.Error(500, fmt.Sprintf("FormFile: %v", err))
		return
	}
	defer file.Close()

	buf := make([]byte, 1024)
	n, _ := file.Read(buf)
	if n > 0 {
		buf = buf[:n]
	}
	fileType := http.DetectContentType(buf)

	if len(setting.Repository.Upload.AllowedTypes) > 0 {
		allowed := false
		for _, t := range setting.Repository.Upload.AllowedTypes {
			t := strings.Trim(t, " ")
			if t == "*/*" || t == fileType {
				allowed = true
				break
			}
		}

		if !allowed {
			ctx.Error(400, ErrFileTypeForbidden.Error())
			return
		}
	}

	upload, err := models.NewUpload(header.Filename, buf, file)
	if err != nil {
		ctx.Error(500, fmt.Sprintf("NewUpload: %v", err))
		return
	}

	log.Trace("New file uploaded: %s", upload.UUID)
	ctx.JSON(200, map[string]string{
		"uuid": upload.UUID,
	})
}

// RemoveUploadFileFromServer remove file from server file dir
func RemoveUploadFileFromServer(ctx *context.Context, form auth.RemoveUploadFileForm) {
	if len(form.File) == 0 {
		ctx.Status(204)
		return
	}

	if err := models.DeleteUploadByUUID(form.File); err != nil {
		ctx.Error(500, fmt.Sprintf("DeleteUploadByUUID: %v", err))
		return
	}

	log.Trace("Upload file removed: %s", form.File)
	ctx.Status(204)
}
