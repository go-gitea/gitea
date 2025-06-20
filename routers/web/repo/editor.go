// Copyright 2016 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/context/upload"
	"code.gitea.io/gitea/services/forms"
	files_service "code.gitea.io/gitea/services/repository/files"
)

const (
	tplEditFile        templates.TplName = "repo/editor/edit"
	tplEditDiffPreview templates.TplName = "repo/editor/diff_preview"
	tplDeleteFile      templates.TplName = "repo/editor/delete"
	tplUploadFile      templates.TplName = "repo/editor/upload"
	tplPatchFile       templates.TplName = "repo/editor/patch"
	tplCherryPick      templates.TplName = "repo/editor/cherry_pick"

	editorCommitChoiceDirect    string = "direct"
	editorCommitChoiceNewBranch string = "commit-to-new-branch"
)

type EditorCommitFormOptions struct {
	CommitFormBehaviors *context.CommitFormBehaviors
}

func prepareEditorCommitFormOptions(ctx *context.Context, editorAction string) *EditorCommitFormOptions {
	// Check if the filename (and additional path) is specified in the querystring
	// (filename is a misnomer, but kept for compatibility with GitHub)
	queryFilename := ctx.Req.URL.Query().Get("filename")
	if queryFilename != "" {
		newTreePath := path.Join(ctx.Repo.TreePath, queryFilename)
		ctx.Redirect(fmt.Sprintf("%s/%s/%s/%s", ctx.Repo.RepoLink, editorAction, util.PathEscapeSegments(ctx.Repo.BranchName), util.PathEscapeSegments(newTreePath)))
		return nil
	}

	cleanedTreePath := files_service.CleanGitTreePath(ctx.Repo.TreePath)
	if cleanedTreePath != ctx.Repo.TreePath {
		ctx.Redirect(fmt.Sprintf("%s/%s/%s/%s", ctx.Repo.RepoLink, editorAction, util.PathEscapeSegments(ctx.Repo.BranchName), util.PathEscapeSegments(cleanedTreePath)))
		return nil
	}
	ctx.Repo.TreePath = cleanedTreePath

	commitFormBehaviors, err := ctx.Repo.PrepareCommitFormBehaviors(ctx, ctx.Doer)
	if err != nil {
		ctx.ServerError("PrepareCommitFormBehaviors", err)
		return nil
	}
	opts := &EditorCommitFormOptions{
		CommitFormBehaviors: commitFormBehaviors,
	}

	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/" + ctx.Repo.RefTypeNameSubURL()
	ctx.Data["TreePath"] = ctx.Repo.TreePath
	ctx.Data["TreeNames"], ctx.Data["TreePaths"] = getParentTreeFields(ctx.Repo.TreePath)
	ctx.Data["CommitFormBehaviors"] = commitFormBehaviors
	ctx.Data["CommitFormOptions"] = opts

	// for online editor
	ctx.Data["PreviewableExtensions"] = strings.Join(markup.PreviewableExtensions(), ",")
	ctx.Data["LineWrapExtensions"] = strings.Join(setting.Repository.Editor.LineWrapExtensions, ",")
	ctx.Data["IsEditingFileOnly"] = ctx.FormString("return_uri") != ""
	ctx.Data["ReturnURI"] = ctx.FormString("return_uri")

	ctx.Data["commit_summary"] = ""
	ctx.Data["commit_message"] = ""
	ctx.Data["commit_choice"] = util.Iif(opts.CommitFormBehaviors.CanCommitToBranch, editorCommitChoiceDirect, editorCommitChoiceNewBranch)
	ctx.Data["new_branch_name"] = getUniquePatchBranchName(ctx, ctx.Doer.LowerName, ctx.Repo.Repository)
	ctx.Data["last_commit"] = ctx.Repo.CommitID

	return opts
}

// redirectForCommitChoice redirects after committing the edit to a branch
func redirectForCommitChoice(ctx *context.Context, formOpts *EditorCommitFormOptions, commitChoice, newBranchName, treePath string) {
	if commitChoice == editorCommitChoiceNewBranch {
		// Redirect to a pull request when possible
		redirectToPullRequest := false
		repo, baseBranch, headBranch := ctx.Repo.Repository, ctx.Repo.BranchName, newBranchName
		if repo.UnitEnabled(ctx, unit.TypePullRequests) {
			redirectToPullRequest = true
		} else if formOpts.CommitFormBehaviors.CanCreateBasePullRequest {
			redirectToPullRequest = true
			baseBranch = repo.BaseRepo.DefaultBranch
			headBranch = repo.Owner.Name + "/" + repo.Name + ":" + headBranch
			repo = repo.BaseRepo
		}
		if redirectToPullRequest {
			ctx.JSONRedirect(repo.Link() + "/compare/" + util.PathEscapeSegments(baseBranch) + "..." + util.PathEscapeSegments(headBranch))
			return
		}
	}

	returnURI := ctx.FormString("return_uri")
	if returnURI == "" || !httplib.IsCurrentGiteaSiteURL(ctx, returnURI) {
		returnURI = ctx.Repo.RepoLink + "/src/branch/" + util.PathEscapeSegments(newBranchName) + "/" + util.PathEscapeSegments(treePath)
	}
	ctx.JSONRedirect(returnURI)
}

func editFileOpenExisting(ctx *context.Context) (prefetch []byte, dataRc io.ReadCloser, fInfo *fileInfo) {
	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(ctx.Repo.TreePath)
	if err != nil {
		HandleGitError(ctx, "GetTreeEntryByPath", err)
		return nil, nil, nil
	}

	// No way to edit a directory online.
	if entry.IsDir() {
		ctx.NotFound(nil)
		return nil, nil, nil
	}

	blob := entry.Blob()
	buf, dataRc, fInfo, err := getFileReader(ctx, ctx.Repo.Repository.ID, blob)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("getFileReader", err)
		}
		return nil, nil, nil
	}

	if fInfo.isLFSFile {
		lfsLock, err := git_model.GetTreePathLock(ctx, ctx.Repo.Repository.ID, ctx.Repo.TreePath)
		if err != nil {
			_ = dataRc.Close()
			ctx.ServerError("GetTreePathLock", err)
			return nil, nil, nil
		} else if lfsLock != nil && lfsLock.OwnerID != ctx.Doer.ID {
			_ = dataRc.Close()
			ctx.NotFound(nil)
			return nil, nil, nil
		}
	}

	return buf, dataRc, fInfo
}

func editFile(ctx *context.Context, editorAction string) {
	isNewFile := editorAction == "_new"
	ctx.Data["IsNewFile"] = isNewFile

	_ = prepareEditorCommitFormOptions(ctx, editorAction)
	if ctx.Written() {
		return
	}

	if !isNewFile {
		prefetch, dataRc, fInfo := editFileOpenExisting(ctx)
		if ctx.Written() {
			return
		}
		defer dataRc.Close()

		ctx.Data["FileSize"] = fInfo.fileSize

		// Only some file types are editable online as text.
		if fInfo.isLFSFile {
			ctx.Data["NotEditableReason"] = ctx.Tr("repo.editor.cannot_edit_lfs_files")
		} else if !fInfo.st.IsRepresentableAsText() {
			ctx.Data["NotEditableReason"] = ctx.Tr("repo.editor.cannot_edit_non_text_files")
		} else if fInfo.fileSize >= setting.UI.MaxDisplayFileSize {
			ctx.Data["NotEditableReason"] = ctx.Tr("repo.editor.cannot_edit_too_large_file")
		}

		if ctx.Data["NotEditableReason"] == nil {
			buf, err := io.ReadAll(io.MultiReader(bytes.NewReader(prefetch), dataRc))
			if err != nil {
				ctx.ServerError("ReadAll", err)
				return
			}
			if content, err := charset.ToUTF8(buf, charset.ConvertOpts{KeepBOM: true}); err != nil {
				ctx.Data["FileContent"] = string(buf)
			} else {
				ctx.Data["FileContent"] = content
			}
		}
	}

	ctx.Data["EditorconfigJson"] = getContextRepoEditorConfig(ctx, ctx.Repo.TreePath)
	ctx.HTML(http.StatusOK, tplEditFile)
}

// EditFile render edit file page
func EditFile(ctx *context.Context) {
	editFile(ctx, "_edit")
}

// NewFile render create file page
func NewFile(ctx *context.Context) {
	editFile(ctx, "_new")
}

func editFilePost(ctx *context.Context, form *forms.EditRepoFileForm, editorAction string) {
	form.TreePath = files_service.CleanGitTreePath(form.TreePath)
	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	isNewFile := editorAction == "_new"
	formOpts := prepareEditorCommitFormOptions(ctx, editorAction)
	if ctx.Written() {
		return
	}

	branchName := util.Iif(form.CommitChoice == editorCommitChoiceNewBranch, form.NewBranchName, ctx.Repo.BranchName)
	if branchName == ctx.Repo.BranchName && !formOpts.CommitFormBehaviors.CanCommitToBranch {
		ctx.JSONError(ctx.Tr("repo.editor.cannot_commit_to_protected_branch", branchName))
		return
	}

	defaultMessage := util.Iif(isNewFile, ctx.Locale.TrString("repo.editor.add", form.TreePath), ctx.Locale.TrString("repo.editor.update", form.TreePath))
	commitMessage := buildEditorCommitMessage(defaultMessage, form.CommitSummary, form.CommitMessage)

	// Committer user info
	gitCommitter, valid := WebGitOperationGetCommitChosenEmailIdentity(ctx, form.CommitEmail)
	if !valid {
		ctx.JSONError(ctx.Tr("repo.editor.invalid_commit_email"))
		return
	}

	var operation string
	if isNewFile {
		operation = "create"
	} else if form.Content.Has() {
		// The form content only has data if the file is representable as text, is not too large and not in lfs.
		operation = "update"
	} else if ctx.Repo.TreePath != form.TreePath {
		// If it doesn't have data, the only possible operation is a "rename"
		operation = "rename"
	} else {
		// It should never happen, just in case
		ctx.JSONError(ctx.Tr("error.occurred"))
		return
	}

	_, err := files_service.ChangeRepoFiles(ctx, ctx.Repo.Repository, ctx.Doer, &files_service.ChangeRepoFilesOptions{
		LastCommitID: form.LastCommit,
		OldBranch:    ctx.Repo.BranchName,
		NewBranch:    branchName,
		Message:      commitMessage,
		Files: []*files_service.ChangeRepoFile{
			{
				Operation:     operation,
				FromTreePath:  ctx.Repo.TreePath,
				TreePath:      form.TreePath,
				ContentReader: strings.NewReader(strings.ReplaceAll(form.Content.Value(), "\r", "")),
			},
		},
		Signoff:   form.Signoff,
		Author:    gitCommitter,
		Committer: gitCommitter,
	})
	if err != nil {
		editorHandleFileOperationError(ctx, branchName, err)
		return
	}

	redirectForCommitChoice(ctx, formOpts, form.CommitChoice, branchName, form.TreePath)
}

func EditFilePost(ctx *context.Context) {
	editFilePost(ctx, web.GetForm(ctx).(*forms.EditRepoFileForm), "_edit")
}

func NewFilePost(ctx *context.Context) {
	editFilePost(ctx, web.GetForm(ctx).(*forms.EditRepoFileForm), "_new")
}

// DeleteFile render delete file page
func DeleteFile(ctx *context.Context) {
	_ = prepareEditorCommitFormOptions(ctx, "_delete")
	if ctx.Written() {
		return
	}
	ctx.Data["PageIsDelete"] = true
	ctx.HTML(http.StatusOK, tplDeleteFile)
}

// DeleteFilePost response for deleting file
func DeleteFilePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.DeleteRepoFileForm)
	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	formOpts := prepareEditorCommitFormOptions(ctx, "_delete")

	branchName := util.Iif(form.CommitChoice == editorCommitChoiceNewBranch, form.NewBranchName, ctx.Repo.BranchName)
	if branchName == ctx.Repo.BranchName && !formOpts.CommitFormBehaviors.CanCommitToBranch {
		ctx.JSONError(ctx.Tr("repo.editor.cannot_commit_to_protected_branch", branchName))
		return
	}

	commitMessage := buildEditorCommitMessage(ctx.Locale.TrString("repo.editor.delete", ctx.Repo.TreePath), form.CommitSummary, form.CommitMessage)

	gitCommitter, valid := WebGitOperationGetCommitChosenEmailIdentity(ctx, form.CommitEmail)
	if !valid {
		ctx.JSONError(ctx.Tr("repo.editor.invalid_commit_email"))
		return
	}

	_, err := files_service.ChangeRepoFiles(ctx, ctx.Repo.Repository, ctx.Doer, &files_service.ChangeRepoFilesOptions{
		LastCommitID: form.LastCommit,
		OldBranch:    ctx.Repo.BranchName,
		NewBranch:    branchName,
		Files: []*files_service.ChangeRepoFile{
			{
				Operation: "delete",
				TreePath:  ctx.Repo.TreePath,
			},
		},
		Message:   commitMessage,
		Signoff:   form.Signoff,
		Author:    gitCommitter,
		Committer: gitCommitter,
	})
	if err != nil {
		editorHandleFileOperationError(ctx, branchName, err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.editor.file_delete_success", ctx.Repo.TreePath))
	redirectTreePath := getClosestParentWithFiles(ctx.Repo.GitRepo, ctx.Repo.BranchName, ctx.Repo.TreePath)
	redirectForCommitChoice(ctx, formOpts, form.CommitChoice, branchName, redirectTreePath)
}

// UploadFile render upload file page
func UploadFile(ctx *context.Context) {
	ctx.Data["PageIsUpload"] = true
	upload.AddUploadContext(ctx, "repo")
	_ = prepareEditorCommitFormOptions(ctx, "_upload")
	if ctx.Written() {
		return
	}
	ctx.HTML(http.StatusOK, tplUploadFile)
}

// UploadFilePost response for uploading file
func UploadFilePost(ctx *context.Context) {
	ctx.Data["PageIsUpload"] = true

	form := web.GetForm(ctx).(*forms.UploadRepoFileForm)
	form.TreePath = files_service.CleanGitTreePath(form.TreePath)
	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	formOpts := prepareEditorCommitFormOptions(ctx, "_upload")
	if ctx.Written() {
		return
	}

	oldBranchName := ctx.Repo.BranchName

	branchName := util.Iif(form.CommitChoice == editorCommitChoiceNewBranch, form.NewBranchName, ctx.Repo.BranchName)

	if oldBranchName != branchName {
		if exist, err := git_model.IsBranchExist(ctx, ctx.Repo.Repository.ID, branchName); err == nil && exist {
			ctx.JSONError(ctx.Tr("repo.editor.branch_already_exists", branchName))
			return
		}
	} else if !formOpts.CommitFormBehaviors.CanCommitToBranch {
		ctx.JSONError(ctx.Tr("repo.editor.cannot_commit_to_protected_branch", branchName))
		return
	}

	commitMessage := buildEditorCommitMessage(ctx.Locale.TrString("repo.editor.upload_files_to_dir", util.IfZero(form.TreePath, "/")), form.CommitSummary, form.CommitMessage)

	gitCommitter, valid := WebGitOperationGetCommitChosenEmailIdentity(ctx, form.CommitEmail)
	if !valid {
		ctx.JSONError(ctx.Tr("repo.editor.invalid_commit_email"))
		return
	}

	err := files_service.UploadRepoFiles(ctx, ctx.Repo.Repository, ctx.Doer, &files_service.UploadRepoFileOptions{
		LastCommitID: form.LastCommit,
		OldBranch:    oldBranchName,
		NewBranch:    branchName,
		TreePath:     form.TreePath,
		Message:      commitMessage,
		Files:        form.Files,
		Signoff:      form.Signoff,
		Author:       gitCommitter,
		Committer:    gitCommitter,
	})
	if err != nil {
		editorHandleFileOperationError(ctx, branchName, err)
		return
	}

	redirectForCommitChoice(ctx, formOpts, form.CommitChoice, branchName, form.TreePath)
}
