// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/repository/files"
)

func CherryPick(ctx *context.Context) {
	_ = prepareEditorCommitFormOptions(ctx, "_cherrypick")
	if ctx.Written() {
		return
	}

	fromCommitID := ctx.PathParam("sha")
	ctx.Data["FromCommitID"] = fromCommitID
	cherryPickCommit, err := ctx.Repo.GitRepo.GetCommit(fromCommitID)
	if err != nil {
		HandleGitError(ctx, "GetCommit", err)
		return
	}

	if ctx.FormString("cherry-pick-type") == "revert" {
		ctx.Data["CherryPickType"] = "revert"
		ctx.Data["commit_summary"] = "revert " + ctx.PathParam("sha")
		ctx.Data["commit_message"] = "revert " + cherryPickCommit.Message()
	} else {
		ctx.Data["CherryPickType"] = "cherry-pick"
		splits := strings.SplitN(cherryPickCommit.Message(), "\n", 2)
		ctx.Data["commit_summary"] = splits[0]
		ctx.Data["commit_message"] = splits[1]
	}

	ctx.HTML(http.StatusOK, tplCherryPick)
}

func CherryPickPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CherryPickForm)
	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	formOpts := prepareEditorCommitFormOptions(ctx, "_cherrypick")
	if ctx.Written() {
		return
	}

	fromCommitID := ctx.PathParam("sha")

	branchName := util.Iif(form.CommitChoice == editorCommitChoiceNewBranch, form.NewBranchName, ctx.Repo.BranchName)
	if branchName == ctx.Repo.BranchName && !formOpts.CommitFormBehaviors.CanCommitToBranch {
		ctx.JSONError(ctx.Tr("repo.editor.cannot_commit_to_protected_branch", branchName))
		return
	}

	defaultMessage := util.Iif(form.Revert, ctx.Locale.TrString("repo.commit.revert-header", fromCommitID), ctx.Locale.TrString("repo.commit.cherry-pick-header", fromCommitID))
	commitMessage := buildEditorCommitMessage(defaultMessage, form.CommitSummary, form.CommitMessage)

	gitCommitter, valid := WebGitOperationGetCommitChosenEmailIdentity(ctx, form.CommitEmail)
	if !valid {
		ctx.JSONError(ctx.Tr("repo.editor.invalid_commit_email"))
		return
	}

	opts := &files.ApplyDiffPatchOptions{
		LastCommitID: form.LastCommit,
		OldBranch:    ctx.Repo.BranchName,
		NewBranch:    branchName,
		Message:      commitMessage,
		Author:       gitCommitter,
		Committer:    gitCommitter,
	}

	// First try the simple plain read-tree -m approach
	opts.Content = fromCommitID
	if _, err := files.CherryPick(ctx, ctx.Repo.Repository, ctx.Doer, form.Revert, opts); err != nil {
		// Drop through to the "apply" method
		buf := &bytes.Buffer{}
		if form.Revert {
			err = git.GetReverseRawDiff(ctx, ctx.Repo.Repository.RepoPath(), fromCommitID, buf)
		} else {
			err = git.GetRawDiff(ctx.Repo.GitRepo, fromCommitID, "patch", buf)
		}
		if err == nil {
			opts.Content = buf.String()
			_, err = files.ApplyDiffPatch(ctx, ctx.Repo.Repository, ctx.Doer, opts)
		}
		if err != nil {
			editorHandleFileOperationError(ctx, branchName, err)
			return
		}
	}

	if form.CommitChoice == editorCommitChoiceNewBranch && ctx.Repo.Repository.UnitEnabled(ctx, unit.TypePullRequests) {
		ctx.JSONRedirect(ctx.Repo.RepoLink + "/compare/" + util.PathEscapeSegments(ctx.Repo.BranchName) + "..." + util.PathEscapeSegments(form.NewBranchName))
	} else {
		ctx.JSONRedirect(ctx.Repo.RepoLink + "/src/branch/" + util.PathEscapeSegments(branchName))
	}
}
