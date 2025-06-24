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
	"code.gitea.io/gitea/modules/log"
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

func prepareEditorCommitFormOptions(ctx *context.Context, editorAction string) *context.CommitFormOptions {
	cleanedTreePath := files_service.CleanGitTreePath(ctx.Repo.TreePath)
	if cleanedTreePath != ctx.Repo.TreePath {
		redirectTo := fmt.Sprintf("%s/%s/%s/%s", ctx.Repo.RepoLink, editorAction, util.PathEscapeSegments(ctx.Repo.BranchName), util.PathEscapeSegments(cleanedTreePath))
		if ctx.Req.URL.RawQuery != "" {
			redirectTo += "?" + ctx.Req.URL.RawQuery
		}
		ctx.Redirect(redirectTo)
		return nil
	}

	commitFormOptions, err := context.PrepareCommitFormOptions(ctx, ctx.Doer, ctx.Repo.Repository, ctx.Repo.Permission, ctx.Repo.RefFullName)
	if err != nil {
		ctx.ServerError("PrepareCommitFormOptions", err)
		return nil
	}

	if commitFormOptions.NeedFork {
		ForkToEdit(ctx)
		return nil
	}

	if commitFormOptions.WillSubmitToFork && !commitFormOptions.TargetRepo.CanEnableEditor() {
		ctx.Data["NotFoundPrompt"] = ctx.Locale.Tr("repo.editor.fork_not_editable")
		ctx.NotFound(nil)
	}

	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/" + ctx.Repo.RefTypeNameSubURL()
	ctx.Data["TreePath"] = ctx.Repo.TreePath
	ctx.Data["CommitFormOptions"] = commitFormOptions

	// for online editor
	ctx.Data["PreviewableExtensions"] = strings.Join(markup.PreviewableExtensions(), ",")
	ctx.Data["LineWrapExtensions"] = strings.Join(setting.Repository.Editor.LineWrapExtensions, ",")
	ctx.Data["IsEditingFileOnly"] = ctx.FormString("return_uri") != ""
	ctx.Data["ReturnURI"] = ctx.FormString("return_uri")

	// form fields
	ctx.Data["commit_summary"] = ""
	ctx.Data["commit_message"] = ""
	ctx.Data["commit_choice"] = util.Iif(commitFormOptions.CanCommitToBranch, editorCommitChoiceDirect, editorCommitChoiceNewBranch)
	ctx.Data["new_branch_name"] = getUniquePatchBranchName(ctx, ctx.Doer.LowerName, commitFormOptions.TargetRepo)
	ctx.Data["last_commit"] = ctx.Repo.CommitID
	return commitFormOptions
}

func prepareTreePathFieldsAndPaths(ctx *context.Context, treePath string) {
	// show the tree path fields in the "breadcrumb" and help users to edit the target tree path
	ctx.Data["TreeNames"], ctx.Data["TreePaths"] = getParentTreeFields(strings.TrimPrefix(treePath, "/"))
}

type preparedEditorCommitForm[T any] struct {
	form              T
	commonForm        *forms.CommitCommonForm
	CommitFormOptions *context.CommitFormOptions
	OldBranchName     string
	NewBranchName     string
	GitCommitter      *files_service.IdentityOptions
}

func (f *preparedEditorCommitForm[T]) GetCommitMessage(defaultCommitMessage string) string {
	commitMessage := util.IfZero(strings.TrimSpace(f.commonForm.CommitSummary), defaultCommitMessage)
	if body := strings.TrimSpace(f.commonForm.CommitMessage); body != "" {
		commitMessage += "\n\n" + body
	}
	return commitMessage
}

func prepareEditorCommitSubmittedForm[T forms.CommitCommonFormInterface](ctx *context.Context) *preparedEditorCommitForm[T] {
	form := web.GetForm(ctx).(T)
	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return nil
	}

	commonForm := form.GetCommitCommonForm()
	commonForm.TreePath = files_service.CleanGitTreePath(commonForm.TreePath)

	commitFormOptions, err := context.PrepareCommitFormOptions(ctx, ctx.Doer, ctx.Repo.Repository, ctx.Repo.Permission, ctx.Repo.RefFullName)
	if err != nil {
		ctx.ServerError("PrepareCommitFormOptions", err)
		return nil
	}
	if commitFormOptions.NeedFork {
		// It shouldn't happen, because we should have done the checks in the "GET" request. But just in case.
		ctx.JSONError(ctx.Locale.TrString("error.not_found"))
		return nil
	}

	// check commit behavior
	fromBaseBranch := ctx.FormString("from_base_branch")
	commitToNewBranch := commonForm.CommitChoice == editorCommitChoiceNewBranch || fromBaseBranch != ""
	targetBranchName := util.Iif(commitToNewBranch, commonForm.NewBranchName, ctx.Repo.BranchName)
	if targetBranchName == ctx.Repo.BranchName && !commitFormOptions.CanCommitToBranch {
		ctx.JSONError(ctx.Tr("repo.editor.cannot_commit_to_protected_branch", targetBranchName))
		return nil
	}

	// Committer user info
	gitCommitter, valid := WebGitOperationGetCommitChosenEmailIdentity(ctx, commonForm.CommitEmail)
	if !valid {
		ctx.JSONError(ctx.Tr("repo.editor.invalid_commit_email"))
		return nil
	}

	if commitToNewBranch {
		// if target branch exists, we should stop
		targetBranchExists, err := git_model.IsBranchExist(ctx, commitFormOptions.TargetRepo.ID, targetBranchName)
		if err != nil {
			ctx.ServerError("IsBranchExist", err)
			return nil
		} else if targetBranchExists {
			if fromBaseBranch != "" {
				ctx.JSONError(ctx.Tr("repo.editor.fork_branch_exists", targetBranchName))
			} else {
				ctx.JSONError(ctx.Tr("repo.editor.branch_already_exists", targetBranchName))
			}
			return nil
		}
	}

	oldBranchName := ctx.Repo.BranchName
	if fromBaseBranch != "" {
		err = editorPushBranchToForkedRepository(ctx, ctx.Doer, ctx.Repo.Repository.BaseRepo, fromBaseBranch, commitFormOptions.TargetRepo, targetBranchName)
		if err != nil {
			log.Error("Unable to editorPushBranchToForkedRepository: %v", err)
			ctx.JSONError(ctx.Tr("repo.editor.fork_failed_to_push_branch", targetBranchName))
			return nil
		}
		// we have pushed the base branch as the new branch, now we need to commit the changes directly to the new branch
		oldBranchName = targetBranchName
	}

	return &preparedEditorCommitForm[T]{
		form:              form,
		commonForm:        commonForm,
		CommitFormOptions: commitFormOptions,
		OldBranchName:     oldBranchName,
		NewBranchName:     targetBranchName,
		GitCommitter:      gitCommitter,
	}
}

// redirectForCommitChoice redirects after committing the edit to a branch
func redirectForCommitChoice[T any](ctx *context.Context, parsed *preparedEditorCommitForm[T], treePath string) {
	// when editing a file in a PR, it should return to the origin location
	if returnURI := ctx.FormString("return_uri"); returnURI != "" && httplib.IsCurrentGiteaSiteURL(ctx, returnURI) {
		ctx.JSONRedirect(returnURI)
		return
	}

	if parsed.commonForm.CommitChoice == editorCommitChoiceNewBranch {
		// Redirect to a pull request when possible
		redirectToPullRequest := false
		repo, baseBranch, headBranch := ctx.Repo.Repository, parsed.OldBranchName, parsed.NewBranchName
		if ctx.Repo.Repository.IsFork && parsed.CommitFormOptions.CanCreateBasePullRequest {
			redirectToPullRequest = true
			baseBranch = repo.BaseRepo.DefaultBranch
			headBranch = repo.Owner.Name + "/" + repo.Name + ":" + headBranch
			repo = repo.BaseRepo
		} else if repo.UnitEnabled(ctx, unit.TypePullRequests) {
			redirectToPullRequest = true
		}
		if redirectToPullRequest {
			ctx.JSONRedirect(repo.Link() + "/compare/" + util.PathEscapeSegments(baseBranch) + "..." + util.PathEscapeSegments(headBranch))
			return
		}
	}

	// redirect to the newly updated file
	redirectTo := util.URLJoin(ctx.Repo.RepoLink, "src/branch", util.PathEscapeSegments(parsed.NewBranchName), util.PathEscapeSegments(treePath))
	ctx.JSONRedirect(redirectTo)
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

func EditFile(ctx *context.Context) {
	editorAction := ctx.PathParam("editor_action")
	isNewFile := editorAction == "_new"
	ctx.Data["IsNewFile"] = isNewFile

	// Check if the filename (and additional path) is specified in the querystring
	// (filename is a misnomer, but kept for compatibility with GitHub)
	urlQuery := ctx.Req.URL.Query()
	queryFilename := urlQuery.Get("filename")
	if queryFilename != "" {
		newTreePath := path.Join(ctx.Repo.TreePath, queryFilename)
		redirectTo := fmt.Sprintf("%s/%s/%s/%s", ctx.Repo.RepoLink, editorAction, util.PathEscapeSegments(ctx.Repo.BranchName), util.PathEscapeSegments(newTreePath))
		urlQuery.Del("filename")
		if newQueryParams := urlQuery.Encode(); newQueryParams != "" {
			redirectTo += "?" + newQueryParams
		}
		ctx.Redirect(redirectTo)
		return
	}

	// on the "New File" page, we should add an empty path field to make end users could input a new name
	prepareTreePathFieldsAndPaths(ctx, util.Iif(isNewFile, ctx.Repo.TreePath+"/", ctx.Repo.TreePath))

	prepareEditorCommitFormOptions(ctx, editorAction)
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

func EditFilePost(ctx *context.Context) {
	editorAction := ctx.PathParam("editor_action")
	isNewFile := editorAction == "_new"
	parsed := prepareEditorCommitSubmittedForm[*forms.EditRepoFileForm](ctx)
	if ctx.Written() {
		return
	}

	defaultCommitMessage := util.Iif(isNewFile, ctx.Locale.TrString("repo.editor.add", parsed.form.TreePath), ctx.Locale.TrString("repo.editor.update", parsed.form.TreePath))

	var operation string
	if isNewFile {
		operation = "create"
	} else if parsed.form.Content.Has() {
		// The form content only has data if the file is representable as text, is not too large and not in lfs.
		operation = "update"
	} else if ctx.Repo.TreePath != parsed.form.TreePath {
		// If it doesn't have data, the only possible operation is a "rename"
		operation = "rename"
	} else {
		// It should never happen, just in case
		ctx.JSONError(ctx.Tr("error.occurred"))
		return
	}

	_, err := files_service.ChangeRepoFiles(ctx, ctx.Repo.Repository, ctx.Doer, &files_service.ChangeRepoFilesOptions{
		LastCommitID: parsed.form.LastCommit,
		OldBranch:    parsed.OldBranchName,
		NewBranch:    parsed.NewBranchName,
		Message:      parsed.GetCommitMessage(defaultCommitMessage),
		Files: []*files_service.ChangeRepoFile{
			{
				Operation:     operation,
				FromTreePath:  ctx.Repo.TreePath,
				TreePath:      parsed.form.TreePath,
				ContentReader: strings.NewReader(strings.ReplaceAll(parsed.form.Content.Value(), "\r", "")),
			},
		},
		Signoff:   parsed.form.Signoff,
		Author:    parsed.GitCommitter,
		Committer: parsed.GitCommitter,
	})
	if err != nil {
		editorHandleFileOperationError(ctx, parsed.NewBranchName, err)
		return
	}

	redirectForCommitChoice(ctx, parsed, parsed.form.TreePath)
}

// DeleteFile render delete file page
func DeleteFile(ctx *context.Context) {
	prepareEditorCommitFormOptions(ctx, "_delete")
	if ctx.Written() {
		return
	}
	ctx.Data["PageIsDelete"] = true
	ctx.HTML(http.StatusOK, tplDeleteFile)
}

// DeleteFilePost response for deleting file
func DeleteFilePost(ctx *context.Context) {
	parsed := prepareEditorCommitSubmittedForm[*forms.DeleteRepoFileForm](ctx)
	if ctx.Written() {
		return
	}

	treePath := ctx.Repo.TreePath
	_, err := files_service.ChangeRepoFiles(ctx, ctx.Repo.Repository, ctx.Doer, &files_service.ChangeRepoFilesOptions{
		LastCommitID: parsed.form.LastCommit,
		OldBranch:    parsed.OldBranchName,
		NewBranch:    parsed.NewBranchName,
		Files: []*files_service.ChangeRepoFile{
			{
				Operation: "delete",
				TreePath:  treePath,
			},
		},
		Message:   parsed.GetCommitMessage(ctx.Locale.TrString("repo.editor.delete", treePath)),
		Signoff:   parsed.form.Signoff,
		Author:    parsed.GitCommitter,
		Committer: parsed.GitCommitter,
	})
	if err != nil {
		editorHandleFileOperationError(ctx, parsed.NewBranchName, err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.editor.file_delete_success", treePath))
	redirectTreePath := getClosestParentWithFiles(ctx.Repo.GitRepo, parsed.NewBranchName, treePath)
	redirectForCommitChoice(ctx, parsed, redirectTreePath)
}

func UploadFile(ctx *context.Context) {
	ctx.Data["PageIsUpload"] = true
	prepareTreePathFieldsAndPaths(ctx, ctx.Repo.TreePath)
	opts := prepareEditorCommitFormOptions(ctx, "_upload")
	if ctx.Written() {
		return
	}
	upload.AddUploadContextForRepo(ctx, opts.TargetRepo)

	ctx.HTML(http.StatusOK, tplUploadFile)
}

func UploadFilePost(ctx *context.Context) {
	parsed := prepareEditorCommitSubmittedForm[*forms.UploadRepoFileForm](ctx)
	if ctx.Written() {
		return
	}

	defaultCommitMessage := ctx.Locale.TrString("repo.editor.upload_files_to_dir", util.IfZero(parsed.form.TreePath, "/"))
	err := files_service.UploadRepoFiles(ctx, ctx.Repo.Repository, ctx.Doer, &files_service.UploadRepoFileOptions{
		LastCommitID: parsed.form.LastCommit,
		OldBranch:    parsed.OldBranchName,
		NewBranch:    parsed.NewBranchName,
		TreePath:     parsed.form.TreePath,
		Message:      parsed.GetCommitMessage(defaultCommitMessage),
		Files:        parsed.form.Files,
		Signoff:      parsed.form.Signoff,
		Author:       parsed.GitCommitter,
		Committer:    parsed.GitCommitter,
	})
	if err != nil {
		editorHandleFileOperationError(ctx, parsed.NewBranchName, err)
		return
	}
	redirectForCommitChoice(ctx, parsed, parsed.form.TreePath)
}
