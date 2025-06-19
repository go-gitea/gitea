// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/repository/files"
)

func NewDiffPatch(ctx *context.Context) {
	_ = prepareEditorCommitFormOptions(ctx, "_diffpatch")
	if ctx.Written() {
		return
	}
	ctx.Data["PageIsPatch"] = true
	ctx.HTML(http.StatusOK, tplPatchFile)
}

// NewDiffPatchPost response for sending patch page
func NewDiffPatchPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.EditRepoFileForm)
	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	formOpts := prepareEditorCommitFormOptions(ctx, "_diffpatch")
	if ctx.Written() {
		return
	}

	branchName := util.Iif(form.CommitChoice == editorCommitChoiceNewBranch, form.NewBranchName, ctx.Repo.BranchName)
	if branchName == ctx.Repo.BranchName && !formOpts.CommitFormBehaviors.CanCommitToBranch {
		ctx.JSONError(ctx.Tr("repo.editor.cannot_commit_to_protected_branch", branchName))
		return
	}

	commitMessage := buildEditorCommitMessage(ctx.Locale.TrString("repo.editor.patch"), form.CommitSummary, form.CommitMessage)

	gitCommitter, valid := WebGitOperationGetCommitChosenEmailIdentity(ctx, form.CommitEmail)
	if !valid {
		ctx.Data["Err_CommitEmail"] = true
		ctx.RenderWithErr(ctx.Tr("repo.editor.invalid_commit_email"), tplPatchFile, &form)
		return
	}

	fileResponse, err := files.ApplyDiffPatch(ctx, ctx.Repo.Repository, ctx.Doer, &files.ApplyDiffPatchOptions{
		LastCommitID: form.LastCommit,
		OldBranch:    ctx.Repo.BranchName,
		NewBranch:    branchName,
		Message:      commitMessage,
		Content:      strings.ReplaceAll(form.Content.Value(), "\r", ""),
		Author:       gitCommitter,
		Committer:    gitCommitter,
	})
	if err != nil {
		editorHandleFileOperationError(ctx, branchName, err)
		return
	}

	if form.CommitChoice == editorCommitChoiceNewBranch && ctx.Repo.Repository.UnitEnabled(ctx, unit.TypePullRequests) {
		ctx.JSONRedirect(ctx.Repo.RepoLink + "/compare/" + util.PathEscapeSegments(ctx.Repo.BranchName) + "..." + util.PathEscapeSegments(form.NewBranchName))
	} else {
		ctx.JSONRedirect(ctx.Repo.RepoLink + "/commit/" + fileResponse.Commit.SHA)
	}
}
