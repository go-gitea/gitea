// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/repository/files"
)

func CherryPick(ctx *context.Context) {
	prepareEditorCommitFormOptions(ctx, "_cherrypick")
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
	fromCommitID := ctx.PathParam("sha")
	parsed := prepareEditorCommitSubmittedForm[*forms.CherryPickForm](ctx)
	if ctx.Written() {
		return
	}

	defaultCommitMessage := util.Iif(parsed.form.Revert, ctx.Locale.TrString("repo.commit.revert-header", fromCommitID), ctx.Locale.TrString("repo.commit.cherry-pick-header", fromCommitID))
	opts := &files.ApplyDiffPatchOptions{
		LastCommitID: parsed.form.LastCommit,
		OldBranch:    parsed.OldBranchName,
		NewBranch:    parsed.NewBranchName,
		Message:      parsed.GetCommitMessage(defaultCommitMessage),
		Author:       parsed.GitCommitter,
		Committer:    parsed.GitCommitter,
	}

	// First try the simple plain read-tree -m approach
	opts.Content = fromCommitID
	if _, err := files.CherryPick(ctx, ctx.Repo.Repository, ctx.Doer, parsed.form.Revert, opts); err != nil {
		// Drop through to the "apply" method
		buf := &bytes.Buffer{}
		if parsed.form.Revert {
			err = git.GetReverseRawDiff(ctx, ctx.Repo.Repository.RepoPath(), fromCommitID, buf)
		} else {
			err = git.GetRawDiff(ctx.Repo.GitRepo, fromCommitID, "patch", buf)
		}
		if err == nil {
			opts.Content = buf.String()
			_, err = files.ApplyDiffPatch(ctx, ctx.Repo.Repository, ctx.Doer, opts)
			if err != nil {
				err = util.ErrorWrapLocale(err, "repo.editor.fail_to_apply_patch")
			}
		}
		if err != nil {
			editorHandleFileOperationError(ctx, parsed.NewBranchName, err)
			return
		}
	}
	redirectForCommitChoice(ctx, parsed, parsed.form.TreePath)
}
