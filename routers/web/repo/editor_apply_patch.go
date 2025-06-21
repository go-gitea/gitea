// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/repository/files"
)

func NewDiffPatch(ctx *context.Context) {
	prepareEditorCommitFormOptions(ctx, "_diffpatch")
	if ctx.Written() {
		return
	}

	ctx.Data["PageIsPatch"] = true
	ctx.HTML(http.StatusOK, tplPatchFile)
}

// NewDiffPatchPost response for sending patch page
func NewDiffPatchPost(ctx *context.Context) {
	parsed := prepareEditorCommitSubmittedForm[*forms.EditRepoFileForm](ctx)
	if ctx.Written() {
		return
	}

	defaultCommitMessage := ctx.Locale.TrString("repo.editor.patch")
	_, err := files.ApplyDiffPatch(ctx, ctx.Repo.Repository, ctx.Doer, &files.ApplyDiffPatchOptions{
		LastCommitID: parsed.form.LastCommit,
		OldBranch:    parsed.OldBranchName,
		NewBranch:    parsed.NewBranchName,
		Message:      parsed.GetCommitMessage(defaultCommitMessage),
		Content:      strings.ReplaceAll(parsed.form.Content.Value(), "\r\n", "\n"),
		Author:       parsed.GitCommitter,
		Committer:    parsed.GitCommitter,
	})
	if err != nil {
		err = util.ErrorWrapLocale(err, "repo.editor.fail_to_apply_patch")
	}
	if err != nil {
		editorHandleFileOperationError(ctx, parsed.NewBranchName, err)
		return
	}
	redirectForCommitChoice(ctx, parsed, parsed.form.TreePath)
}
