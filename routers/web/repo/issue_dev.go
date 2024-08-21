// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	repo_service "code.gitea.io/gitea/services/repository"
)

func CreateBranchFromIssue(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if issue.IsPull {
		ctx.Flash.Error(ctx.Tr("repo.issues.create_branch_from_issue_error_is_pull"))
		ctx.Redirect(issue.Link(), http.StatusSeeOther)
		return
	}

	form := web.GetForm(ctx).(*forms.NewBranchForm)
	if !ctx.Repo.CanCreateBranch() {
		ctx.NotFound("CreateBranch", nil)
		return
	}

	if ctx.HasError() {
		ctx.Flash.Error(ctx.GetErrMsg())
		ctx.Redirect(ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL())
		return
	}

	if err := repo_service.CreateNewBranch(ctx, ctx.Doer, ctx.Repo.Repository, ctx.Repo.GitRepo, form.SourceBranchName, form.NewBranchName); err != nil {
		handleCreateBranchError(ctx, err, form)
		return
	}

	if err := issues_model.CreateIssueDevLink(ctx, &issues_model.IssueDevLink{
		IssueID:   issue.ID,
		LinkType:  issues_model.IssueDevLinkTypeBranch,
		LinkIndex: form.NewBranchName,
	}); err != nil {
		ctx.ServerError("CreateIssueDevLink", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.issues.create_branch_from_issue_success", ctx.Repo.BranchName))
	ctx.Redirect(issue.Link())
}
