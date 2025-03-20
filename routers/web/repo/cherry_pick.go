// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"errors"
	"strings"

	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/repository/files"
)

var tplCherryPick templates.TplName = "repo/editor/cherry_pick"

// CherryPick handles cherrypick GETs
func CherryPick(ctx *context.Context) {
	ctx.Data["SHA"] = ctx.PathParam("sha")
	cherryPickCommit, err := ctx.Repo.GitRepo.GetCommit(ctx.PathParam("sha"))
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound(err)
			return
		}
		ctx.ServerError("GetCommit", err)
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

	canCommit := renderCommitRights(ctx)
	ctx.Data["TreePath"] = ""

	if canCommit {
		ctx.Data["commit_choice"] = frmCommitChoiceDirect
	} else {
		ctx.Data["commit_choice"] = frmCommitChoiceNewBranch
	}
	ctx.Data["new_branch_name"] = GetUniquePatchBranchName(ctx)
	ctx.Data["last_commit"] = ctx.Repo.CommitID
	ctx.Data["LineWrapExtensions"] = strings.Join(setting.Repository.Editor.LineWrapExtensions, ",")
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/" + ctx.Repo.RefTypeNameSubURL()

	ctx.HTML(200, tplCherryPick)
}

// CherryPickPost handles cherrypick POSTs
func CherryPickPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CherryPickForm)

	sha := ctx.PathParam("sha")
	ctx.Data["SHA"] = sha
	if form.Revert {
		ctx.Data["CherryPickType"] = "revert"
	} else {
		ctx.Data["CherryPickType"] = "cherry-pick"
	}

	canCommit := renderCommitRights(ctx)
	branchName := ctx.Repo.BranchName
	if form.CommitChoice == frmCommitChoiceNewBranch {
		branchName = form.NewBranchName
	}
	ctx.Data["commit_summary"] = form.CommitSummary
	ctx.Data["commit_message"] = form.CommitMessage
	ctx.Data["commit_choice"] = form.CommitChoice
	ctx.Data["new_branch_name"] = form.NewBranchName
	ctx.Data["last_commit"] = ctx.Repo.CommitID
	ctx.Data["LineWrapExtensions"] = strings.Join(setting.Repository.Editor.LineWrapExtensions, ",")
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/" + ctx.Repo.RefTypeNameSubURL()

	if ctx.HasError() {
		ctx.HTML(200, tplCherryPick)
		return
	}

	// Cannot commit to a an existing branch if user doesn't have rights
	if branchName == ctx.Repo.BranchName && !canCommit {
		ctx.Data["Err_NewBranchName"] = true
		ctx.Data["commit_choice"] = frmCommitChoiceNewBranch
		ctx.RenderWithErr(ctx.Tr("repo.editor.cannot_commit_to_protected_branch", branchName), tplCherryPick, &form)
		return
	}

	message := strings.TrimSpace(form.CommitSummary)
	if message == "" {
		if form.Revert {
			message = ctx.Locale.TrString("repo.commit.revert-header", sha)
		} else {
			message = ctx.Locale.TrString("repo.commit.cherry-pick-header", sha)
		}
	}

	form.CommitMessage = strings.TrimSpace(form.CommitMessage)
	if len(form.CommitMessage) > 0 {
		message += "\n\n" + form.CommitMessage
	}

	gitCommitter, valid := WebGitOperationGetCommitChosenEmailIdentity(ctx, form.CommitEmail)
	if !valid {
		ctx.Data["Err_CommitEmail"] = true
		ctx.RenderWithErr(ctx.Tr("repo.editor.invalid_commit_email"), tplCherryPick, &form)
		return
	}
	opts := &files.ApplyDiffPatchOptions{
		LastCommitID: form.LastCommit,
		OldBranch:    ctx.Repo.BranchName,
		NewBranch:    branchName,
		Message:      message,
		Author:       gitCommitter,
		Committer:    gitCommitter,
	}

	// First lets try the simple plain read-tree -m approach
	opts.Content = sha
	if _, err := files.CherryPick(ctx, ctx.Repo.Repository, ctx.Doer, form.Revert, opts); err != nil {
		if git_model.IsErrBranchAlreadyExists(err) {
			// User has specified a branch that already exists
			branchErr := err.(git_model.ErrBranchAlreadyExists)
			ctx.Data["Err_NewBranchName"] = true
			ctx.RenderWithErr(ctx.Tr("repo.editor.branch_already_exists", branchErr.BranchName), tplCherryPick, &form)
			return
		} else if files.IsErrCommitIDDoesNotMatch(err) {
			ctx.RenderWithErr(ctx.Tr("repo.editor.file_changed_while_editing", ctx.Repo.RepoLink+"/compare/"+form.LastCommit+"..."+ctx.Repo.CommitID), tplPatchFile, &form)
			return
		}
		// Drop through to the apply technique

		buf := &bytes.Buffer{}
		if form.Revert {
			if err := git.GetReverseRawDiff(ctx, ctx.Repo.Repository.RepoPath(), sha, buf); err != nil {
				if git.IsErrNotExist(err) {
					ctx.NotFound(errors.New("commit " + ctx.PathParam("sha") + " does not exist."))
					return
				}
				ctx.ServerError("GetRawDiff", err)
				return
			}
		} else {
			if err := git.GetRawDiff(ctx.Repo.GitRepo, sha, git.RawDiffType("patch"), buf); err != nil {
				if git.IsErrNotExist(err) {
					ctx.NotFound(errors.New("commit " + ctx.PathParam("sha") + " does not exist."))
					return
				}
				ctx.ServerError("GetRawDiff", err)
				return
			}
		}

		opts.Content = buf.String()
		ctx.Data["FileContent"] = opts.Content

		if _, err := files.ApplyDiffPatch(ctx, ctx.Repo.Repository, ctx.Doer, opts); err != nil {
			if git_model.IsErrBranchAlreadyExists(err) {
				// User has specified a branch that already exists
				branchErr := err.(git_model.ErrBranchAlreadyExists)
				ctx.Data["Err_NewBranchName"] = true
				ctx.RenderWithErr(ctx.Tr("repo.editor.branch_already_exists", branchErr.BranchName), tplCherryPick, &form)
				return
			} else if files.IsErrCommitIDDoesNotMatch(err) {
				ctx.RenderWithErr(ctx.Tr("repo.editor.file_changed_while_editing", ctx.Repo.RepoLink+"/compare/"+form.LastCommit+"..."+ctx.Repo.CommitID), tplPatchFile, &form)
				return
			}
			ctx.RenderWithErr(ctx.Tr("repo.editor.fail_to_apply_patch", err), tplPatchFile, &form)
			return
		}
	}

	if form.CommitChoice == frmCommitChoiceNewBranch && ctx.Repo.Repository.UnitEnabled(ctx, unit.TypePullRequests) {
		ctx.Redirect(ctx.Repo.RepoLink + "/compare/" + util.PathEscapeSegments(ctx.Repo.BranchName) + "..." + util.PathEscapeSegments(form.NewBranchName))
	} else {
		ctx.Redirect(ctx.Repo.RepoLink + "/src/branch/" + util.PathEscapeSegments(branchName))
	}
}
