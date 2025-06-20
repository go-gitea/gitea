// Copyright 2025 Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"

	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/utils"
	context_service "code.gitea.io/gitea/services/context"
	files_service "code.gitea.io/gitea/services/repository/files"
)

func errorAs[T error](v error) (e T, ok bool) {
	if errors.As(v, &e) {
		return e, true
	}
	return e, false
}

func editorHandleFileOperationErrorRender(ctx *context_service.Context, message, summary, details string) {
	flashError, err := ctx.RenderToHTML(tplAlertDetails, map[string]any{
		"Message": message,
		"Summary": summary,
		"Details": utils.SanitizeFlashErrorString(details),
	})
	if err == nil {
		ctx.JSONError(flashError)
	} else {
		log.Error("RenderToHTML: %v", err)
		ctx.JSONError(message + "\n" + summary + "\n" + utils.SanitizeFlashErrorString(details))
	}
}

func editorHandleFileOperationError(ctx *context_service.Context, targetBranchName string, err error) {
	if errAs := util.ErrorAsLocale(err); errAs != nil {
		ctx.JSONError(ctx.Tr(errAs.TrKey, errAs.TrArgs...))
	} else if errAs, ok := errorAs[git.ErrNotExist](err); ok {
		ctx.JSONError(ctx.Tr("repo.editor.file_modifying_no_longer_exists", errAs.RelPath))
	} else if errAs, ok := errorAs[git_model.ErrLFSFileLocked](err); ok {
		ctx.JSONError(ctx.Tr("repo.editor.upload_file_is_locked", errAs.Path, errAs.UserName))
	} else if errAs, ok := errorAs[files_service.ErrFilenameInvalid](err); ok {
		ctx.JSONError(ctx.Tr("repo.editor.filename_is_invalid", errAs.Path))
	} else if errAs, ok := errorAs[files_service.ErrFilePathInvalid](err); ok {
		switch errAs.Type {
		case git.EntryModeSymlink:
			ctx.JSONError(ctx.Tr("repo.editor.file_is_a_symlink", errAs.Path))
		case git.EntryModeTree:
			ctx.JSONError(ctx.Tr("repo.editor.filename_is_a_directory", errAs.Path))
		case git.EntryModeBlob:
			ctx.JSONError(ctx.Tr("repo.editor.directory_is_a_file", errAs.Path))
		default:
			ctx.JSONError(ctx.Tr("repo.editor.filename_is_invalid", errAs.Path))
		}
	} else if errAs, ok := errorAs[files_service.ErrRepoFileAlreadyExists](err); ok {
		ctx.JSONError(ctx.Tr("repo.editor.file_already_exists", errAs.Path))
	} else if errAs, ok := errorAs[git.ErrBranchNotExist](err); ok {
		ctx.JSONError(ctx.Tr("repo.editor.branch_does_not_exist", errAs.Name))
	} else if errAs, ok := errorAs[git_model.ErrBranchAlreadyExists](err); ok {
		ctx.JSONError(ctx.Tr("repo.editor.branch_already_exists", errAs.BranchName))
	} else if files_service.IsErrCommitIDDoesNotMatch(err) {
		ctx.JSONError(ctx.Tr("repo.editor.commit_id_not_matching"))
	} else if files_service.IsErrCommitIDDoesNotMatch(err) || git.IsErrPushOutOfDate(err) {
		ctx.JSONError(ctx.Tr("repo.editor.file_changed_while_editing", ctx.Repo.RepoLink+"/compare/"+util.PathEscapeSegments(ctx.Repo.CommitID)+"..."+util.PathEscapeSegments(targetBranchName)))
	} else if errAs, ok := errorAs[*git.ErrPushRejected](err); ok {
		if errAs.Message == "" {
			ctx.JSONError(ctx.Tr("repo.editor.push_rejected_no_message"))
		} else {
			editorHandleFileOperationErrorRender(ctx, ctx.Locale.TrString("repo.editor.push_rejected"), ctx.Locale.TrString("repo.editor.push_rejected_summary"), errAs.Message)
		}
	} else if errors.Is(err, util.ErrNotExist) {
		ctx.JSONError(ctx.Tr("error.not_found"))
	} else {
		setting.PanicInDevOrTesting("unclear err %T: %v", err, err)
		editorHandleFileOperationErrorRender(ctx, ctx.Locale.TrString("repo.editor.failed_to_commit"), ctx.Locale.TrString("repo.editor.failed_to_commit_summary"), err.Error())
	}
}
