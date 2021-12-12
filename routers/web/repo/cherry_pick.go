// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"bytes"
	"errors"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/repository/files"
)

var tplCherryPick base.TplName = "repo/editor/cherry_pick"

func refCherryPick(ctx *context.Context) {
	var err error
	refName := ctx.FormString("ref")
	ctx.Repo.RefName = refName
	ctx.Repo.IsViewBranch = true
	ctx.Repo.BranchName = ctx.Repo.RefName
	ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetBranchCommit(refName)
	if err != nil {
		ctx.ServerError("GetBranchCommit", err)
		return
	}
	ctx.Repo.CommitID = ctx.Repo.Commit.ID.String()

	ctx.Data["BranchName"] = ctx.Repo.BranchName
	ctx.Data["BranchNameSubURL"] = ctx.Repo.BranchNameSubURL()
	ctx.Data["TagName"] = ctx.Repo.TagName
	ctx.Data["CommitID"] = ctx.Repo.CommitID
	ctx.Data["TreePath"] = ctx.Repo.TreePath
	ctx.Data["IsViewBranch"] = ctx.Repo.IsViewBranch
	ctx.Data["IsViewTag"] = ctx.Repo.IsViewTag
	ctx.Data["IsViewCommit"] = ctx.Repo.IsViewCommit
	ctx.Data["CanCreateBranch"] = ctx.Repo.CanCreateBranch()

	ctx.Repo.CommitsCount, err = ctx.Repo.GetCommitsCount()
	if err != nil {
		ctx.ServerError("GetCommitsCount", err)
		return
	}
	ctx.Data["CommitsCount"] = ctx.Repo.CommitsCount

	if !ctx.Repo.Repository.CanEnableEditor() || ctx.Repo.IsViewCommit {
		ctx.NotFound("", nil)
		return
	}

}

func CherryPick(ctx *context.Context) {
	ctx.Data["SHA"] = ctx.Params(":sha")
	if ctx.FormString("cherry-pick-type") == "revert" {
		ctx.Data["CherryPickType"] = "revert"
		ctx.Data["commit_summary"] = "revert " + ctx.Params(":sha")
		ctx.Data["commit_message"] = ""
	} else {
		ctx.Data["CherryPickType"] = "cherry-pick"
		ctx.Data["commit_summary"] = "cherry-pick " + ctx.Params(":sha")
		ctx.Data["commit_message"] = ""
	}

	ctx.Data["RequireHighlightJS"] = true

	canCommit := renderCommitRights(ctx)
	ctx.Data["TreePath"] = "patch"

	if canCommit {
		ctx.Data["commit_choice"] = frmCommitChoiceDirect
	} else {
		ctx.Data["commit_choice"] = frmCommitChoiceNewBranch
	}
	ctx.Data["new_branch_name"] = GetUniquePatchBranchName(ctx)
	ctx.Data["last_commit"] = ctx.Repo.CommitID
	ctx.Data["LineWrapExtensions"] = strings.Join(setting.Repository.Editor.LineWrapExtensions, ",")
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL()

	ctx.HTML(200, tplCherryPick)
}

func CherryPickPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CherryPickForm)

	sha := ctx.Params(":sha")
	ctx.Data["SHA"] = sha
	if form.Revert {
		ctx.Data["CherryPickType"] = "revert"
		ctx.Data["commit_summary"] = "revert " + ctx.Params(":sha")
		ctx.Data["commit_message"] = ""
	} else {
		ctx.Data["CherryPickType"] = "cherry-pick"
		ctx.Data["commit_summary"] = "cherry-pick " + ctx.Params(":sha")
		ctx.Data["commit_message"] = ""
	}

	ctx.Data["RequireHighlightJS"] = true
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
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL()

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
			message = ctx.Tr("repo.commit.revert-header", sha)
		} else {
			message = ctx.Tr("repo.commit.cherry-pick-header", sha)
		}
	}

	form.CommitMessage = strings.TrimSpace(form.CommitMessage)
	if len(form.CommitMessage) > 0 {
		message += "\n\n" + form.CommitMessage
	}

	opts := &files.ApplyDiffPatchOptions{
		LastCommitID: form.LastCommit,
		OldBranch:    ctx.Repo.BranchName,
		NewBranch:    branchName,
		Message:      message,
	}

	buf := &bytes.Buffer{}
	if form.Revert {
		if err := git.GetReverseRawDiff(
			ctx,
			ctx.Repo.Repository.RepoPath(),
			sha,
			buf,
		); err != nil {
			if git.IsErrNotExist(err) {
				ctx.NotFound("GetRawDiff",
					errors.New("commit "+ctx.Params(":sha")+" does not exist."))
				return
			}
			ctx.ServerError("GetRawDiff", err)
			return
		}
	} else {
		if err := git.GetRawDiff(
			ctx,
			ctx.Repo.Repository.RepoPath(),
			sha,
			git.RawDiffType("patch"),
			buf,
		); err != nil {
			if git.IsErrNotExist(err) {
				ctx.NotFound("GetRawDiff",
					errors.New("commit "+ctx.Params(":sha")+" does not exist."))
				return
			}
			ctx.ServerError("GetRawDiff", err)
			return
		}
	}
	opts.Content = buf.String()
	ctx.Data["FileContent"] = opts.Content

	if _, err := files.ApplyDiffPatch(ctx.Repo.Repository, ctx.User, opts); err != nil {
		if models.IsErrBranchAlreadyExists(err) {
			// For when a user specifies a new branch that already exists
			ctx.Data["Err_NewBranchName"] = true
			if branchErr, ok := err.(models.ErrBranchAlreadyExists); ok {
				ctx.RenderWithErr(ctx.Tr("repo.editor.branch_already_exists", branchErr.BranchName), tplCherryPick, &form)
			} else {
				ctx.Error(500, err.Error())
			}
		} else if models.IsErrCommitIDDoesNotMatch(err) {
			ctx.RenderWithErr(ctx.Tr("repo.editor.file_changed_while_editing", ctx.Repo.RepoLink+"/compare/"+form.LastCommit+"..."+ctx.Repo.CommitID), tplPatchFile, &form)
		} else {
			ctx.RenderWithErr(ctx.Tr("repo.editor.fail_to_apply_patch", err), tplPatchFile, &form)
		}
	}

	if form.CommitChoice == frmCommitChoiceNewBranch && ctx.Repo.Repository.UnitEnabled(unit.TypePullRequests) {
		ctx.Redirect(ctx.Repo.RepoLink + "/compare/" + util.PathEscapeSegments(ctx.Repo.BranchName) + "..." + util.PathEscapeSegments(form.NewBranchName))
	} else {
		ctx.Redirect(ctx.Repo.RepoLink + "/src/branch/" + util.PathEscapeSegments(branchName))
	}

}
